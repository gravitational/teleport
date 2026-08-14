package mock

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const tokenEndpoint = "/a/idm/auth/oauth2/token"
const tokenRevocationEndpoint = "/a/idm/auth/oauth2/revoke"

func (s *Server) handleOSPToken(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	type openIDConfig struct {
		TokenEndpoint      string `json:"token_endpoint"`
		RevocationEndpoint string `json:"revocation_endpoint"`
	}
	config := openIDConfig{
		TokenEndpoint:      s.BaseOSPURL + tokenEndpoint,
		RevocationEndpoint: s.BaseOSPURL + tokenRevocationEndpoint,
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
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		Refreshtoken string `json:"refresh_token"`
	}

	count := s.loginCount.Add(1)

	token := tokenResponse{
		AccessToken:  s.authToken,
		TokenType:    "Bearer",
		ExpiresIn:    30,
		Refreshtoken: RefreshToken(s.authToken, int(count)),
	}
	_ = writeJSON(w, token)
}

func RefreshToken(authToken string, count int) string {
	return authToken + fmt.Sprintf("-refresh-%d", count)
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

func (s *Server) handleOSPRevoke(w http.ResponseWriter, r *http.Request) {
	if err := s.validateRevokeRequest(r); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = writeJSON(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)

}

func (s *Server) validateRevokeRequest(r *http.Request) *ErrorPayload {
	username, password, ok := r.BasicAuth()
	if !ok {
		return newUnauthenticatedError("missing basic auth")
	}
	if username != s.OAuthClientID || password != s.OAuthClientSecret {
		return newUnauthenticatedError("invalid basic auth credentials")
	}

	grantType := r.FormValue("token_type_hint")
	if grantType != "refresh_token" {
		return newUnauthenticatedError("invalid token type hint")
	}

	token := r.FormValue("token")

	s.revokedTokensMu.Lock()
	defer s.revokedTokensMu.Unlock()
	s.revokedTokens = append(s.revokedTokens, token)

	return nil
}
