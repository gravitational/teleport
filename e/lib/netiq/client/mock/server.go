package mock

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"time"
)

// Server is a mock server for testing the client.
type Server struct {
	*httptest.Server
	BaseOSPURL string
	BaseAPIURL string

	IdentityVaultUser     string
	IdentityVaultPassword string
	OAuthClientID         string
	OAuthClientSecret     string

	roles           []Role
	users           []User
	groups          []Group
	groupMembers    map[string][]GroupMember
	roleMembers     map[string][]RoleAssignmentStatus
	roleParents     map[string][]RoleRef
	mappedResources map[string][]ResourceRef
	resources       []Resource

	authToken       string
	loginCount      atomic.Int32
	revokedTokens   []string
	revokedTokensMu sync.Mutex
}

// New creates a new mock server.
func New() *Server {
	router := http.NewServeMux()
	httpServer := httptest.NewUnstartedServer(router)
	server := &Server{
		Server:          httpServer,
		roles:           defaultRoles(),
		users:           defaultUsers(),
		groups:          defaultGroups(),
		groupMembers:    defaultGroupMembers(),
		roleMembers:     defaultRoleMembers(),
		roleParents:     defaultRoleParentRefs(),
		resources:       defaultResources(),
		mappedResources: defaultMappedResources(),
	}
	server.fillUsersAndPasswords()

	router.HandleFunc("GET /osp/a/idm/auth/oauth2/.well-known/openid-configuration", server.handleOSPToken)
	router.HandleFunc("POST /osp/a/idm/auth/oauth2/token", server.handleOSPAuth)
	router.HandleFunc("POST /osp/a/idm/auth/oauth2/revoke", server.handleOSPRevoke)
	router.HandleFunc("GET /api/rest/access/users/list", server.handleGetUsers)
	router.HandleFunc("GET /api/rest/catalog/groups", server.handleGetGroups)
	router.HandleFunc("POST /api/rest/access/groups/members", server.handleGetGroupMembers)
	router.HandleFunc("GET /api/rest/catalog/roles/listV2", server.handleGetRoles)
	router.HandleFunc("POST /api/rest/catalog/roles/role/assignments/v2", server.handleGetRoleMembers)
	router.HandleFunc("POST /api/rest/catalog/roles/parentRoles/list", server.handleGetRoleParents)
	router.HandleFunc("GET /api/rest/catalog/resources/listV2", server.handleGetResources)
	router.HandleFunc("POST /api/rest/catalog/roles/mappedResources/list", server.handleGetRoleMappedResources)
	server.StartTLS()

	server.BaseOSPURL = httpServer.URL + "/osp"
	server.BaseAPIURL = httpServer.URL + "/api"
	return server
}

func (s *Server) GetRevokedTokens() []string {
	s.revokedTokensMu.Lock()
	defer s.revokedTokensMu.Unlock()
	return s.revokedTokens
}

func (s *Server) ActiveTokens() int {
	s.revokedTokensMu.Lock()
	defer s.revokedTokensMu.Unlock()
	return int(s.loginCount.Load()) - len(s.revokedTokens)
}

func buildSecureToken(user, password, time string) string {
	const secretToken = "highly_secret_token"
	h := sha256.New()
	h.Write([]byte(secretToken))
	h.Write([]byte(user))
	h.Write([]byte(password))
	h.Write([]byte(time))
	return base64.RawStdEncoding.EncodeToString(h.Sum(nil))
}

func (s *Server) fillUsersAndPasswords() {
	s.IdentityVaultUser = "admin"
	s.IdentityVaultPassword = "password"
	s.OAuthClientID = "client_id"
	s.OAuthClientSecret = "client"
	s.authToken = buildSecureToken(s.IdentityVaultUser, s.IdentityVaultPassword, time.Now().String())
}

func (s *Server) validateBearerToken(r *http.Request) *ErrorPayload {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return newUnauthenticatedError("missing bearer token")
	}
	if authHeader != "Bearer "+s.authToken {
		return newUnauthenticatedError("invalid bearer token" + authHeader)
	}
	return nil
}
