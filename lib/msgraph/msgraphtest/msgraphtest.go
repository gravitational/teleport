package msgraphtest

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/google/uuid"
)

// TokenProvider implements [msgraph.AzureTokenProvider]
type TokenProvider struct {
	mu    sync.Mutex
	Token string
}

// GetToken returns a token to be used in msgraph request.
func (t *TokenProvider) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.Token == "" {
		t.Token = uuid.NewString()
	}

	return azcore.AccessToken{
		Token: t.Token,
	}, nil
}

// ClearToken deletes token value.
func (t *TokenProvider) ClearToken() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Token = ""
}

// InspectToken returns the current token without generating a new one if the current token is
// empty. Useful in tests that need to verify that the client requested a new token after it was
// cleared.
func (t *TokenProvider) InspectToken() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.Token
}

// Payloads defines payload value to fake msgraph responses.
type Payloads struct {
	Users, Groups, Applications string
	GroupMembers                map[string]string
}

// DefaultPayload creates a default response payload.
func DefaultPayload() Payloads {
	return Payloads{
		Users:        PayloadListUsers,
		Groups:       PayloadListGroups,
		Applications: PayloadGetApplication,
		GroupMembers: map[string]string{
			"group1": PayloadListGroup1Members,
			"group2": PayloadListGroup2Members,
			"group3": PayloadListGroup3Members,
		},
	}
}

// Server defines fake server.
type Server struct {
	TokenProvider TokenProvider
	Payloads      Payloads
	TLSServer     *httptest.Server
	HTTPClient    *http.Client
}

// ServerOption is a custom opt for [NewServer].
type ServerOption func(*Server)

// WithPayloads sets custom response payload.
func WithPayloads(p Payloads) ServerOption {
	return func(s *Server) {
		s.Payloads = p
	}
}

// NewServer creates a new fake server.
func NewServer(opts ...ServerOption) *Server {
	s := &Server{
		TokenProvider: TokenProvider{},
		Payloads:      DefaultPayload(),
	}
	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	tlsServer := httptest.NewTLSServer(s.Handler())
	s.TLSServer = tlsServer

	httpClient := tlsServer.Client()
	httpClient.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		// Ignore the address and always direct all requests to the fake API server.
		// This allows tests to connect to the fake API server despite the official
		// client trying to reach the official endpoints.
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("tcp", tlsServer.Listener.Addr().String())
		},
	}
	s.HTTPClient = httpClient

	return s
}

// Fake server handler
func (s *Server) Handler() http.Handler {
	r := http.NewServeMux()

	r.HandleFunc("GET /v1.0/users", s.handleListUsers)
	r.HandleFunc("GET /v1.0/groups", s.handleListGroups)
	r.HandleFunc("GET /v1.0/groups/{groupid}/members", s.handleListGroupMembers)
	r.HandleFunc("/v1.0/", s.handleCatchAll)

	return r
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	var source []json.RawMessage

	w.Header().Set("Content-Type", "application/json")
	if s.Payloads.Users == "" {
		w.Write([]byte(`{"value": []}`))
		return
	}
	if err := json.Unmarshal([]byte(s.Payloads.Users), &source); err != nil {
		http.Error(w, fmt.Sprintf("Failed to unmarshal payload: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	Paginator(w, r, source)
}

func (s *Server) handleListGroups(w http.ResponseWriter, r *http.Request) {
	var source []json.RawMessage

	w.Header().Set("Content-Type", "application/json")
	if s.Payloads.Groups == "" {
		w.Write([]byte(`{"value": []}`))
		return
	}
	if err := json.Unmarshal([]byte(s.Payloads.Groups), &source); err != nil {
		http.Error(w, fmt.Sprintf("Failed to unmarshal payload: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	Paginator(w, r, source)
}

func (s *Server) handleListGroupMembers(w http.ResponseWriter, r *http.Request) {
	var source []json.RawMessage

	w.Header().Set("Content-Type", "application/json")

	if len(s.Payloads.GroupMembers) == 0 {
		w.Write([]byte(`{"value": []}`))
		return
	}

	groupID := r.PathValue("groupid")
	if err := json.Unmarshal([]byte(s.Payloads.GroupMembers[groupID]), &source); err != nil {
		http.Error(w, fmt.Sprintf("Failed to unmarshal payload: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	Paginator(w, r, source)
}

// handleGetApplication handles GET /v1.0/applications(appId='...') requests.
func (s *Server) handleGetApplication(w http.ResponseWriter, r *http.Request, appID string) {
	if appID == "app1" {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(s.Payloads.Applications))
		return
	}

	http.NotFound(w, r)
}

var applicationByAppIDPattern = regexp.MustCompile(`^/v1\.0/applications\(appId='([^']+)'\)$`)

// handleCatchAll handles other endpoints like applications(appId='app-id').
func (s *Server) handleCatchAll(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if matches := applicationByAppIDPattern.FindStringSubmatch(r.URL.Path); matches != nil {
			appID := matches[1]
			s.handleGetApplication(w, r, appID)
			return
		}
	}

	http.NotFound(w, r)
}

// Paginator emulates the Graph API's pagination with the given static set of objects.
func Paginator(w http.ResponseWriter, r *http.Request, values []json.RawMessage) {
	top, err := strconv.Atoi(r.URL.Query().Get("$top"))
	if err != nil {
		http.Error(w, "Expected to get $top parameter", http.StatusInternalServerError)
		return
	}

	skip, _ := strconv.Atoi(r.URL.Query().Get("$skipToken"))

	from, to := skip, skip+top
	if to > len(values) {
		to = len(values)
	}
	page := values[from:to]

	nextLink := *r.URL
	nextLink.Host = r.Host
	nextLink.Scheme = "https"
	vals := nextLink.Query()
	// $skipToken is an opaque value in MS Graph, for testing purposes we use a simple offset.
	vals.Set("$skipToken", strconv.Itoa(top+skip))
	nextLink.RawQuery = vals.Encode()

	response := map[string]any{
		"value": page,
	}

	if skip+top < len(values) {
		response["@odata.nextLink"] = nextLink.String()
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, fmt.Sprintf("Failed to unmarshal payload: %s", err.Error()), http.StatusInternalServerError)
	}
}
