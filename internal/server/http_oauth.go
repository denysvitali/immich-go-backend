package server

import (
	"fmt"
	"net/http"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *Server) handleOAuthBackchannelLogout(w http.ResponseWriter, r *http.Request) {
	req, err := oauthBackchannelLogoutRequestFromHTTP(r)
	if err != nil {
		writeGRPCErrorJSON(w, r, status.Error(codes.InvalidArgument, err.Error()))
		return
	}

	if _, err := s.LogoutOAuth(gatewayIncomingContext(r), req); err != nil {
		writeGRPCErrorJSON(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func oauthBackchannelLogoutRequestFromHTTP(r *http.Request) (*immichv1.OAuthBackchannelLogoutRequest, error) {
	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("invalid form body")
	}

	return &immichv1.OAuthBackchannelLogoutRequest{LogoutToken: r.Form.Get("logout_token")}, nil
}
