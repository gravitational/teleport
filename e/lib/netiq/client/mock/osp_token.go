package mock

import (
	"encoding/json"
	"net/http"
)

const tokenEndpoint = "/a/idm/auth/oauth2/token"

func (s *Server) handleOSPToken(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	type openIDConfig struct {
		TokenEndpoint string `json:"token_endpoint"`
	}
	config := openIDConfig{
		TokenEndpoint: s.BaseOSPURL + tokenEndpoint,
	}
	_ = writeJSON(w, config)
}

func writeJSON(w http.ResponseWriter, v any) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(v)
}

func (s *Server) handleOSPAuth(w http.ResponseWriter, r *http.Request) {
	if err := s.validateAuthenticationRequest(r); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = writeJSON(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)

	type tokenResponse struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}

	token := tokenResponse{
		AccessToken: s.authToken,
		TokenType:   "Bearer",
		ExpiresIn:   3600,
	}
	_ = writeJSON(w, token)

}

func (s *Server) validateAuthenticationRequest(r *http.Request) *ErrorPayload {
	username, password, ok := r.BasicAuth()
	if !ok {
		return newUnauthenticatedError("missing basic auth")
	}
	if username != s.OAuthClientID || password != s.OAuthClientSecret {
		return newUnauthenticatedError("invalid basic auth credentials")
	}

	grantType := r.FormValue("grant_type")
	if grantType != "password" {
		return newUnauthenticatedError("invalid grant type")
	}

	identityVaultUser := r.FormValue("username")
	identityVaultPassword := r.FormValue("password")
	if identityVaultUser != s.IdentityVaultUser || identityVaultPassword != s.IdentityVaultPassword {
		return newUnauthenticatedError("invalid oauth credentials")
	}
	return nil
}
