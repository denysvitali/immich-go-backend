package oauth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/db/pgutil"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// OIDCConfig holds the Immich-style single OIDC provider settings used for
// backchannel logout token validation.
type OIDCConfig struct {
	Enabled          bool
	IssuerURL        string
	ClientID         string
	ClientSecret     string
	SigningAlgorithm string
}

// LogoutClaims are the claims required by OIDC backchannel logout
// (OpenID Connect Back-Channel Logout 1.0).
type LogoutClaims struct {
	jwt.RegisteredClaims
	Sid    string         `json:"sid,omitempty"`
	Events map[string]any `json:"events"`
	Nonce  string         `json:"nonce,omitempty"`
}

const backchannelLogoutEvent = "http://schemas.openid.net/event/backchannel-logout"

var (
	errLogoutTokenInvalid = errors.New("token validation failed")
	jwksCacheMu           sync.Mutex
	jwksCache             = map[string]*cachedJWKS{}
)

type cachedJWKS struct {
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
}

// ValidateLogoutToken verifies an OIDC logout token per upstream Immich /
// OpenID Connect Back-Channel Logout rules and returns sub/sid claims.
func ValidateLogoutToken(cfg OIDCConfig, logoutToken string) (sub, sid string, err error) {
	if strings.TrimSpace(logoutToken) == "" {
		return "", "", fmt.Errorf("%w: empty token", errLogoutTokenInvalid)
	}
	if !cfg.Enabled {
		return "", "", fmt.Errorf("oauth not enabled")
	}

	alg := strings.TrimSpace(cfg.SigningAlgorithm)
	if alg == "" {
		alg = "RS256"
	}

	opts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{alg}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(5 * time.Second),
	}
	if cfg.ClientID != "" {
		opts = append(opts, jwt.WithAudience(cfg.ClientID))
	}
	if issuer := strings.TrimRight(cfg.IssuerURL, "/"); issuer != "" {
		opts = append(opts, jwt.WithIssuer(issuer))
	}
	parser := jwt.NewParser(opts...)

	var claims LogoutClaims
	keyFunc := func(token *jwt.Token) (any, error) {
		if strings.HasPrefix(alg, "HS") {
			if cfg.ClientSecret == "" {
				return nil, fmt.Errorf("client secret required for %s", alg)
			}
			return []byte(cfg.ClientSecret), nil
		}
		// Asymmetric: resolve key from issuer JWKS.
		kid, _ := token.Header["kid"].(string)
		return lookupJWKSKey(cfg.IssuerURL, kid)
	}

	// maxTokenAge 2m: reject tokens with iat too far in the past.
	parsed, err := parser.ParseWithClaims(logoutToken, &claims, keyFunc)
	if err != nil || !parsed.Valid {
		return "", "", fmt.Errorf("%w: %v", errLogoutTokenInvalid, err)
	}

	if claims.IssuedAt != nil {
		if time.Since(claims.IssuedAt.Time) > 2*time.Minute+5*time.Second {
			return "", "", fmt.Errorf("%w: token too old", errLogoutTokenInvalid)
		}
	}

	if claims.Events == nil || claims.Events[backchannelLogoutEvent] == nil {
		return "", "", fmt.Errorf("%w: missing backchannel-logout event claim", errLogoutTokenInvalid)
	}
	if claims.Nonce != "" {
		return "", "", fmt.Errorf("%w: logout token must not contain a nonce", errLogoutTokenInvalid)
	}
	sub = claims.Subject
	sid = claims.Sid
	if sub == "" && sid == "" {
		return "", "", fmt.Errorf("%w: must contain either a sub or a sid claim", errLogoutTokenInvalid)
	}

	return sub, sid, nil
}

// InvalidateOAuthSessions deletes sessions matching the OIDC sub and/or sid
// claims, mirroring Immich's sessionRepository.invalidateOAuth.
func InvalidateOAuthSessions(ctx context.Context, db *sqlc.Queries, oauthID, oauthSid string) ([]string, error) {
	if db == nil {
		return nil, fmt.Errorf("database not configured")
	}
	if oauthID == "" && oauthSid == "" {
		return nil, fmt.Errorf("at least one of oauthSid or oauthId must be present")
	}

	var ids []pgtype.UUID
	var err error
	sidText := pgtype.Text{String: oauthSid, Valid: oauthSid != ""}
	switch {
	case oauthSid != "" && oauthID != "":
		ids, err = db.DeleteSessionsByOAuthSidAndOAuthId(ctx, sqlc.DeleteSessionsByOAuthSidAndOAuthIdParams{
			OauthSid: sidText,
			OauthId:  oauthID,
		})
	case oauthSid != "" && oauthID == "":
		ids, err = db.DeleteSessionsByOAuthSid(ctx, sidText)
	default:
		ids, err = db.DeleteSessionsByOAuthId(ctx, oauthID)
	}
	if err != nil {
		return nil, fmt.Errorf("invalidate oauth sessions: %w", err)
	}

	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id.Valid {
			out = append(out, pgutil.UUIDToString(id))
		}
	}
	return out, nil
}

// lookupJWKSKey fetches (and caches) the issuer's JWKS and returns the key for kid.
func lookupJWKSKey(issuerURL, kid string) (*rsa.PublicKey, error) {
	issuerURL = strings.TrimRight(issuerURL, "/")
	if issuerURL == "" {
		return nil, fmt.Errorf("issuer URL required for JWKS validation")
	}

	jwksCacheMu.Lock()
	defer jwksCacheMu.Unlock()

	entry, ok := jwksCache[issuerURL]
	if !ok || time.Since(entry.fetchedAt) > 15*time.Minute {
		keys, err := fetchJWKS(issuerURL)
		if err != nil {
			return nil, err
		}
		entry = &cachedJWKS{keys: keys, fetchedAt: time.Now()}
		jwksCache[issuerURL] = entry
	}

	if kid != "" {
		if k, ok := entry.keys[kid]; ok {
			return k, nil
		}
		// Refresh once if kid missing.
		keys, err := fetchJWKS(issuerURL)
		if err != nil {
			return nil, err
		}
		entry = &cachedJWKS{keys: keys, fetchedAt: time.Now()}
		jwksCache[issuerURL] = entry
		if k, ok := entry.keys[kid]; ok {
			return k, nil
		}
		return nil, fmt.Errorf("jwks key %q not found", kid)
	}
	// No kid: use first key.
	for _, k := range entry.keys {
		return k, nil
	}
	return nil, fmt.Errorf("jwks empty for issuer %s", issuerURL)
}

func fetchJWKS(issuerURL string) (map[string]*rsa.PublicKey, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Discovery document
	discURL := issuerURL + "/.well-known/openid-configuration"
	resp, err := client.Get(discURL)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oidc discovery status %d", resp.StatusCode)
	}
	var disc struct {
		JWKSURI string `json:"jwks_uri"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&disc); err != nil {
		return nil, fmt.Errorf("oidc discovery parse: %w", err)
	}
	if disc.JWKSURI == "" {
		return nil, fmt.Errorf("jwks_uri missing from discovery")
	}

	jwksResp, err := client.Get(disc.JWKSURI)
	if err != nil {
		return nil, fmt.Errorf("jwks fetch: %w", err)
	}
	defer jwksResp.Body.Close()
	if jwksResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jwks status %d", jwksResp.StatusCode)
	}
	body, err := io.ReadAll(jwksResp.Body)
	if err != nil {
		return nil, err
	}
	var jwks struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(body, &jwks); err != nil {
		return nil, fmt.Errorf("jwks parse: %w", err)
	}

	out := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" || k.N == "" || k.E == "" {
			continue
		}
		pub, err := rsaPublicKeyFromJWK(k.N, k.E)
		if err != nil {
			continue
		}
		keyID := k.Kid
		if keyID == "" {
			keyID = fmt.Sprintf("key-%d", len(out))
		}
		out[keyID] = pub
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no RSA keys in jwks")
	}
	return out, nil
}

func rsaPublicKeyFromJWK(nB64, eB64 string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		return nil, err
	}
	n := new(big.Int).SetBytes(nBytes)
	var e int
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}
	if e == 0 {
		return nil, fmt.Errorf("invalid exponent")
	}
	return &rsa.PublicKey{N: n, E: e}, nil
}
