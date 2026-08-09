package server

import (
	"net/http"
	"strings"
)

func (s *Server) corsHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		allowedOrigin, ok := configuredCORSOrigin(origin, s.config.Server.CORSAllowedOrigins)
		if !ok {
			if isCORSPreflight(r) {
				http.Error(w, "origin is not allowed", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Add("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, x-api-key, x-immich-user-token, x-session-id")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Disposition, Content-Length, Location, Set-Cookie")
		if allowedOrigin != "*" {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		if isCORSPreflight(r) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func configuredCORSOrigin(origin string, allowed []string) (string, bool) {
	for _, candidate := range allowed {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" {
			return "*", true
		}
		if candidate == origin {
			return origin, true
		}
	}
	return "", false
}

func isCORSPreflight(r *http.Request) bool {
	return r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""
}
