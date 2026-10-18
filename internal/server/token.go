package server

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

func (s *Server) validateToken() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		want := sha256.Sum256([]byte(s.token))
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if s.token == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			got := sha256.Sum256([]byte(tok))
			if !ok || subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (s *Server) validateSyncSecret() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload UpdatePayload
			body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
			if err != nil {
				http.Error(w, "invalid request body", http.StatusBadRequest)
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				http.Error(w, "invalid payload json", http.StatusBadRequest)
				return
			}
			if s.syncSecret != "" &&
				subtle.ConstantTimeCompare([]byte(s.syncSecret), []byte(payload.Secret)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			// reset the body
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
		})
	}
}
