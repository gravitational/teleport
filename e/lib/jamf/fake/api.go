package fake

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/e/lib/jamf"
)

// TokenExpiryPeriod is the expiration period for bearer tokens.
const TokenExpiryPeriod = 30 * time.Minute

// User holds credentials for an API user.
type User struct {
	Username string
	Password string
}

// API is a fake implementation for the Jamf PRO API.
type API struct {
	clock clockwork.Clock

	// mu guards all fields below it
	mu           sync.Mutex
	users        []*User
	inventory    []*jamf.ComputerInventory
	issuedTokens map[string]*authToken // key is authToken.Token
}

// Opts are the creation options for [API].
type Opts struct {
	Clock clockwork.Clock
}

// New creates a new fake Jamf API.
func New(opts *Opts) *API {
	if opts == nil {
		opts = &Opts{}
	}

	clock := opts.Clock
	if clock == nil {
		clock = clockwork.NewRealClock()
	}

	return &API{
		clock:        clock,
		issuedTokens: make(map[string]*authToken),
	}
}

func (a *API) SetUsers(users []*User) {
	a.mu.Lock()
	a.users = users
	a.mu.Unlock()
}

func (a *API) SetInventory(inv []*jamf.ComputerInventory) {
	a.mu.Lock()
	a.inventory = inv
	a.mu.Unlock()
}

// Handler returns the http.Handler that implements the REST API.
// Prefix is the path before "/v1". For example, use "/api" to get paths like
// "/api/v1/auth/token" and "/api/v1/auth/keep-alive".
func (a *API) Handler(prefix string) http.Handler {
	return &rootHandler{
		API:    a,
		prefix: prefix,
	}
}

type rootHandler struct {
	*API
	prefix string
}

func (a *rootHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Strip prefix from the path, we route from `/v1` onwards.
	path := strings.TrimPrefix(req.URL.Path, a.prefix)

	// The only unauthorized endpoint is POST /v1/auth/token, anything else
	// requires a bearer token.
	if req.Method == http.MethodPost && path == "/v1/auth/token" {
		a.postAuthToken(w, req)
		return
	}

	// Authorize.
	token, ok := a.isAuthorized(req)
	if !ok {
		a.replyError(w, errorResponse{HTTPStatus: 401})
		return
	}
	req = req.WithContext(context.WithValue(req.Context(), authTokenKey{}, token))

	// Route.
	var handler http.HandlerFunc
	switch req.Method {
	case http.MethodGet:
		if path == "/v1/computers-inventory" {
			handler = a.getComputersInventory
		}
	case http.MethodPost:
		if path == "/v1/auth/keep-alive" {
			handler = a.postAuthKeepAlive
		}
	}
	if handler == nil {
		a.replyError(w, errorResponse{HTTPStatus: 404})
		return
	}

	handler(w, req)
}

// authTokenKey is used to save the current *authToken in the request's context.
type authTokenKey struct{}

func (a *API) isAuthorized(req *http.Request) (*authToken, bool) {
	authzHeader := req.Header.Get("Authorization")
	if authzHeader == "" || !strings.HasPrefix(authzHeader, "Bearer ") {
		return nil, false
	}
	token := strings.TrimPrefix(authzHeader, "Bearer ")
	if token == "" {
		return nil, false
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Token exists?
	issuedToken, ok := a.issuedTokens[token]
	if !ok {
		return nil, false
	}

	// Token expired?
	now := a.clock.Now().UTC()
	if now.After(issuedToken.Expires) {
		delete(a.issuedTokens, token)
		return nil, false
	}

	return issuedToken, true
}

type errorResponse struct {
	HTTPStatus int         `json:"httpStatus"`
	Errors     []*apiError `json:"errors"`
}

type apiError struct {
	Code        string      `json:"code"`
	Description string      `json:"description"`
	ID          string      `json:"id"`
	Field       interface{} `json:"field"` // Only seen as `null`.
}

type authToken struct {
	jamf.AuthToken
	owner string
}

func (a *API) postAuthToken(w http.ResponseWriter, req *http.Request) {
	user, pass, ok := req.BasicAuth()
	if !ok {
		a.replyError(w, errorResponse{HTTPStatus: 401})
		return
	}

	// Find user.
	match := false
	a.mu.Lock()
	for _, u := range a.users {
		if u.Username == user && u.Password == pass {
			match = true
			break
		}
	}
	a.mu.Unlock()
	if !match {
		a.replyError(w, errorResponse{HTTPStatus: 401})
		return
	}

	token, err := a.newAuthToken(user)
	if err != nil {
		// Error not observed in practice.
		a.replyError(w, errorResponse{
			HTTPStatus: 500,
			Errors:     []*apiError{{Description: err.Error()}},
		})
		return
	}

	// Commit token to memory.
	a.mu.Lock()
	a.issuedTokens[token.Token] = token
	a.mu.Unlock()

	// Reply.
	a.replyJSON(w, 200, token)
}

func (a *API) newAuthToken(owner string) (*authToken, error) {
	// An opaque string is good enough for our purposes.
	// Size is arbitrary.
	token := make([]byte, 40)
	if _, err := rand.Read(token); err != nil {
		return nil, fmt.Errorf("reading random bytes: %w", err)
	}

	tokenB64 := base64.StdEncoding.EncodeToString(token)
	expires := a.clock.Now().Add(TokenExpiryPeriod).UTC()
	return &authToken{
		AuthToken: jamf.AuthToken{
			Token:   tokenB64,
			Expires: expires,
		},
		owner: owner,
	}, nil
}

func (a *API) postAuthKeepAlive(w http.ResponseWriter, req *http.Request) {
	currentToken := req.Context().Value(authTokenKey{}).(*authToken)

	newToken, err := a.newAuthToken(currentToken.owner)
	if err != nil {
		// Unexpected. Error not observed in practice.
		a.replyError(w, errorResponse{HTTPStatus: 500})
		return
	}

	a.mu.Lock()

	// Issue new token for user.
	a.issuedTokens[newToken.Token] = newToken

	// Rescind old token.
	delete(a.issuedTokens, currentToken.Token)

	a.mu.Unlock()

	// Reply.
	a.replyJSON(w, 200, newToken)
}

func (a *API) getComputersInventory(w http.ResponseWriter, req *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// TODO(codingllama): Implement pagination and other necessary params.

	a.replyJSON(w, 200, &jamf.GetComputersInventoryResponse{
		TotalCount: len(a.inventory),
		Results:    a.inventory,
	})
}

func (a *API) replyError(w http.ResponseWriter, resp errorResponse) {
	if resp.Errors == nil {
		resp.Errors = []*apiError{} // Marshal empty instead of null.
	}

	a.replyJSON(w, resp.HTTPStatus, resp)
}

func (a *API) replyJSON(w http.ResponseWriter, code int, resp any) {
	body, err := json.Marshal(resp)
	if err != nil {
		log.WithError(err).Warn("Failed to marshal JSON response")
	}

	w.WriteHeader(code)
	w.Write(body)
}
