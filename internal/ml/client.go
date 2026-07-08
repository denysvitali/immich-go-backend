// Package ml provides an HTTP client for the Immich machine-learning service.
//
// The remote API matches the official Immich ML container:
//
//	POST {url}/predict  multipart form with fields "entries" (+ "image" or "text")
//	GET  {url}/ping
//
// Features are gated by configuration; when ML is disabled or unreachable the
// caller is expected to degrade gracefully (metadata search, skip jobs, etc.).
package ml

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// Default models used by Immich when no override is configured.
const (
	DefaultCLIPModel   = "ViT-B-32__openai"
	DefaultFaceModel   = "buffalo_l"
	DefaultMinScore    = 0.7
	DefaultMaxDistance = 0.6
	DefaultTimeout     = 60 * time.Second
)

// Sentinel errors for graceful degradation.
var (
	ErrDisabled    = errors.New("machine learning is disabled")
	ErrUnavailable = errors.New("machine learning service unavailable")
	ErrEmptyInput  = errors.New("empty ML input")
	ErrBadResponse = errors.New("invalid machine learning response")
)

// Config controls the ML HTTP client.
type Config struct {
	// Enabled is the master switch for talking to the ML service.
	Enabled bool
	// URL is the base URL of the Immich ML container (e.g. http://localhost:3003).
	URL string
	// Timeout bounds each HTTP request. Zero uses DefaultTimeout.
	Timeout time.Duration
	// CLIP model name (visual + textual).
	CLIPModel string
	// Face model name (detection + recognition pipeline).
	FaceModel string
	// FaceMinScore filters low-confidence detections.
	FaceMinScore float64
	// FaceMaxDistance is the embedding distance threshold for person matching.
	FaceMaxDistance float64
	// CLIPMaxDistance is the embedding distance threshold for smart search.
	CLIPMaxDistance float64
	// DuplicateMaxDistance is the max CLIP distance for near-duplicate grouping.
	DuplicateMaxDistance float64
}

// Client talks to the Immich machine-learning HTTP API.
type Client struct {
	cfg        Config
	httpClient *http.Client
	baseURL    string
}

// NewClient builds a Client. A nil/disabled config yields a client that always
// returns ErrDisabled so callers can keep a non-nil reference.
func NewClient(cfg Config) *Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultTimeout
	}
	if cfg.CLIPModel == "" {
		cfg.CLIPModel = DefaultCLIPModel
	}
	if cfg.FaceModel == "" {
		cfg.FaceModel = DefaultFaceModel
	}
	if cfg.FaceMinScore <= 0 {
		cfg.FaceMinScore = DefaultMinScore
	}
	if cfg.FaceMaxDistance <= 0 {
		cfg.FaceMaxDistance = 0.5
	}
	if cfg.CLIPMaxDistance <= 0 {
		cfg.CLIPMaxDistance = DefaultMaxDistance
	}
	if cfg.DuplicateMaxDistance <= 0 {
		cfg.DuplicateMaxDistance = 0.01
	}

	base := strings.TrimRight(strings.TrimSpace(cfg.URL), "/")
	return &Client{
		cfg:     cfg,
		baseURL: base,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// Enabled reports whether ML calls should be attempted.
func (c *Client) Enabled() bool {
	return c != nil && c.cfg.Enabled && c.baseURL != ""
}

// Config returns a copy of the client configuration.
func (c *Client) Config() Config {
	if c == nil {
		return Config{}
	}
	return c.cfg
}

// Ping checks that the ML service is reachable.
func (c *Client) Ping(ctx context.Context) error {
	if !c.Enabled() {
		return ErrDisabled
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/ping", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: ping status %d", ErrUnavailable, resp.StatusCode)
	}
	return nil
}

// EncodeImage produces a CLIP visual embedding for the given image bytes.
func (c *Client) EncodeImage(ctx context.Context, image []byte, modelName string) ([]float32, error) {
	if !c.Enabled() {
		return nil, ErrDisabled
	}
	if len(image) == 0 {
		return nil, ErrEmptyInput
	}
	if modelName == "" {
		modelName = c.cfg.CLIPModel
	}
	entries := map[string]any{
		"clip": map[string]any{
			"visual": map[string]any{
				"modelName": modelName,
			},
		},
	}
	raw, err := c.predict(ctx, entries, image, "")
	if err != nil {
		return nil, err
	}
	return parseCLIPEmbedding(raw)
}

// EncodeText produces a CLIP textual embedding for the given query string.
func (c *Client) EncodeText(ctx context.Context, text, modelName string) ([]float32, error) {
	if !c.Enabled() {
		return nil, ErrDisabled
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, ErrEmptyInput
	}
	if modelName == "" {
		modelName = c.cfg.CLIPModel
	}
	entries := map[string]any{
		"clip": map[string]any{
			"textual": map[string]any{
				"modelName": modelName,
			},
		},
	}
	raw, err := c.predict(ctx, entries, nil, text)
	if err != nil {
		return nil, err
	}
	return parseCLIPEmbedding(raw)
}

// BoundingBox is an axis-aligned face region in pixel coordinates.
type BoundingBox struct {
	X1 int32 `json:"x1"`
	Y1 int32 `json:"y1"`
	X2 int32 `json:"x2"`
	Y2 int32 `json:"y2"`
}

// DetectedFace is one face returned by the facial-recognition pipeline.
type DetectedFace struct {
	BoundingBox BoundingBox `json:"boundingBox"`
	Embedding   []float32   `json:"-"`
	// EmbeddingRaw is the JSON-array string from the ML service (kept for
	// direct insertion into pgvector when preferred).
	EmbeddingRaw string  `json:"embedding"`
	Score        float64 `json:"score"`
}

// FaceDetectionResult is the full facial-recognition pipeline response.
type FaceDetectionResult struct {
	Faces       []DetectedFace
	ImageWidth  int
	ImageHeight int
}

// DetectFaces runs face detection + recognition on the given image.
func (c *Client) DetectFaces(ctx context.Context, image []byte, modelName string, minScore float64) (*FaceDetectionResult, error) {
	if !c.Enabled() {
		return nil, ErrDisabled
	}
	if len(image) == 0 {
		return nil, ErrEmptyInput
	}
	if modelName == "" {
		modelName = c.cfg.FaceModel
	}
	if minScore <= 0 {
		minScore = c.cfg.FaceMinScore
	}
	entries := map[string]any{
		"facial-recognition": map[string]any{
			"detection": map[string]any{
				"modelName": modelName,
				"options": map[string]any{
					"minScore": minScore,
				},
			},
			"recognition": map[string]any{
				"modelName": modelName,
			},
		},
	}
	raw, err := c.predict(ctx, entries, image, "")
	if err != nil {
		return nil, err
	}
	return parseFaceDetection(raw)
}

// predict posts a multipart /predict request and returns the raw JSON body.
func (c *Client) predict(ctx context.Context, entries map[string]any, image []byte, text string) (json.RawMessage, error) {
	entriesJSON, err := json.Marshal(entries)
	if err != nil {
		return nil, fmt.Errorf("marshal ML entries: %w", err)
	}

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if err := w.WriteField("entries", string(entriesJSON)); err != nil {
		return nil, fmt.Errorf("write entries field: %w", err)
	}
	switch {
	case len(image) > 0:
		part, err := w.CreateFormFile("image", "asset.jpg")
		if err != nil {
			return nil, fmt.Errorf("create image part: %w", err)
		}
		if _, err := part.Write(image); err != nil {
			return nil, fmt.Errorf("write image part: %w", err)
		}
	case text != "":
		if err := w.WriteField("text", text); err != nil {
			return nil, fmt.Errorf("write text field: %w", err)
		}
	default:
		return nil, ErrEmptyInput
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/predict", &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("read ML response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(respBody))
		if len(msg) > 200 {
			msg = msg[:200] + "..."
		}
		return nil, fmt.Errorf("%w: status %d: %s", ErrUnavailable, resp.StatusCode, msg)
	}
	return json.RawMessage(respBody), nil
}

// parseCLIPEmbedding extracts the float vector from a CLIP predict response.
// Immich ML returns {"clip": "[0.1,0.2,...]", "imageHeight": N, "imageWidth": N}.
func parseCLIPEmbedding(raw json.RawMessage) ([]float32, error) {
	var envelope struct {
		CLIP json.RawMessage `json:"clip"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadResponse, err)
	}
	if len(envelope.CLIP) == 0 || string(envelope.CLIP) == "null" {
		return nil, fmt.Errorf("%w: missing clip field", ErrBadResponse)
	}
	return parseEmbeddingValue(envelope.CLIP)
}

// parseEmbeddingValue accepts either a JSON array of numbers or a JSON string
// containing a JSON array (Immich ML serializes numpy arrays as JSON strings).
func parseEmbeddingValue(raw json.RawMessage) ([]float32, error) {
	raw = json.RawMessage(bytes.TrimSpace(raw))
	if len(raw) == 0 {
		return nil, fmt.Errorf("%w: empty embedding", ErrBadResponse)
	}

	// Already a JSON array: [0.1, 0.2, ...]
	if raw[0] == '[' {
		var floats []float64
		if err := json.Unmarshal(raw, &floats); err != nil {
			return nil, fmt.Errorf("%w: array embedding: %v", ErrBadResponse, err)
		}
		return floats64To32(floats), nil
	}

	// JSON string wrapping the array: "[0.1, 0.2, ...]"
	var asString string
	if err := json.Unmarshal(raw, &asString); err != nil {
		return nil, fmt.Errorf("%w: string embedding: %v", ErrBadResponse, err)
	}
	asString = strings.TrimSpace(asString)
	if asString == "" {
		return nil, fmt.Errorf("%w: empty embedding string", ErrBadResponse)
	}
	var floats []float64
	if err := json.Unmarshal([]byte(asString), &floats); err != nil {
		return nil, fmt.Errorf("%w: nested embedding: %v", ErrBadResponse, err)
	}
	return floats64To32(floats), nil
}

func floats64To32(in []float64) []float32 {
	out := make([]float32, len(in))
	for i, v := range in {
		out[i] = float32(v)
	}
	return out
}

// parseFaceDetection extracts faces from a facial-recognition predict response.
func parseFaceDetection(raw json.RawMessage) (*FaceDetectionResult, error) {
	var envelope struct {
		Faces       json.RawMessage `json:"facial-recognition"`
		ImageWidth  int             `json:"imageWidth"`
		ImageHeight int             `json:"imageHeight"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadResponse, err)
	}

	result := &FaceDetectionResult{
		ImageWidth:  envelope.ImageWidth,
		ImageHeight: envelope.ImageHeight,
		Faces:       []DetectedFace{},
	}
	if len(envelope.Faces) == 0 || string(envelope.Faces) == "null" {
		return result, nil
	}

	// Faces may be [] or, if only detection ran, a different shape — require array.
	if envelope.Faces[0] != '[' {
		return nil, fmt.Errorf("%w: facial-recognition is not an array", ErrBadResponse)
	}

	var rawFaces []struct {
		BoundingBox struct {
			X1 float64 `json:"x1"`
			Y1 float64 `json:"y1"`
			X2 float64 `json:"x2"`
			Y2 float64 `json:"y2"`
		} `json:"boundingBox"`
		Embedding json.RawMessage `json:"embedding"`
		Score     float64         `json:"score"`
	}
	if err := json.Unmarshal(envelope.Faces, &rawFaces); err != nil {
		return nil, fmt.Errorf("%w: faces: %v", ErrBadResponse, err)
	}

	for _, rf := range rawFaces {
		emb, err := parseEmbeddingValue(rf.Embedding)
		if err != nil {
			return nil, err
		}
		embRaw, _ := embeddingToJSONString(emb)
		result.Faces = append(result.Faces, DetectedFace{
			BoundingBox: BoundingBox{
				X1: int32(rf.BoundingBox.X1),
				Y1: int32(rf.BoundingBox.Y1),
				X2: int32(rf.BoundingBox.X2),
				Y2: int32(rf.BoundingBox.Y2),
			},
			Embedding:    emb,
			EmbeddingRaw: embRaw,
			Score:        rf.Score,
		})
	}
	return result, nil
}

// FormatVector converts a float embedding to the pgvector text input form
// "[0.1,0.2,...]" which PostgreSQL casts to vector.
func FormatVector(emb []float32) string {
	if len(emb) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.Grow(len(emb) * 8)
	b.WriteByte('[')
	for i, v := range emb {
		if i > 0 {
			b.WriteByte(',')
		}
		// Use JSON-compatible formatting for stable precision.
		fmt.Fprintf(&b, "%g", v)
	}
	b.WriteByte(']')
	return b.String()
}

func embeddingToJSONString(emb []float32) (string, error) {
	data, err := json.Marshal(emb)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
