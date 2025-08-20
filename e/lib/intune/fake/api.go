package fake

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/e/lib/intune/api"
)

// API holds the implementation details of an HTTP handler for the fake Intune API.
type API struct {
	config Config

	// mu guards all fields below it
	mu             sync.Mutex
	apps           []*api.AppCredentials
	managedDevices []*api.ManagedDevice
	issuedTokens   map[string]*accessToken // key is [accessToken.token]
}

// Config contains values needed by [API].
type Config struct {
	Clock  clockwork.Clock
	Logger *slog.Logger
}

// New returns new [API] with no configured apps.
func New(config Config) *API {
	return &API{
		config:         config,
		issuedTokens:   make(map[string]*accessToken),
		managedDevices: []*api.ManagedDevice{},
	}
}

// SetApps overrides app credentials that can be used to authenticate with the fake API.
func (a *API) SetApps(apps []*api.AppCredentials) {
	a.mu.Lock()
	a.apps = apps
	a.mu.Unlock()
}

// SetManagedDevices copies provided devices and sets it as the inventory in the fake API.
func (a *API) SetManagedDevices(devices []*api.ManagedDevice) {
	a.mu.Lock()
	a.managedDevices = make([]*api.ManagedDevice, 0, len(devices))
	for _, d := range devices {
		if d == nil {
			a.managedDevices = append(a.managedDevices, nil)
			continue
		}

		copiedD := *d
		a.managedDevices = append(a.managedDevices, &copiedD)
	}
	a.mu.Unlock()
}

func (a *API) getTenants() map[string]struct{} {
	a.mu.Lock()
	defer a.mu.Unlock()

	tenants := make(map[string]struct{})

	for _, app := range a.apps {
		tenants[app.Tenant] = struct{}{}
	}

	return tenants
}

// Handler returns a thin wrapper for [API] that implements [http.Handler].
func (a *API) Handler() http.Handler {
	return &rootHandler{
		API: a,
	}
}

type rootHandler struct {
	*API
}

// ServeHTTP handles requests that come to the fake API. In the real Intune API, there are separate
// hosts that handle auth requests and regular API requests, but this fake handler processes both
// types of requests.
func (a *rootHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Handle access token endpoint.
	//
	// In the actual Intune API, token requests are sent to login.microsoftonline.com and API requests
	// are sent to graph.microsoft.com.
	// In the fake API however, all requests are sent to the same host.
	//
	// https://learn.microsoft.com/en-us/graph/auth-v2-service?tabs=http#token-request
	const oauthTokenSuffix = "/oauth2/v2.0/token"
	if tenantRaw, isOauthPath := strings.CutSuffix(req.URL.Path, oauthTokenSuffix); isOauthPath {
		tenant := strings.Trim(tenantRaw, "/")
		a.postAccessToken(w, req, tenant)
		return
	}

	_, ok := a.isAuthorized(req)
	if !ok {
		a.replyError(w, 401, api.GraphErrorResource{
			Code:    "InvalidAuthenticationToken",
			Message: "invalid authentication token",
		})
		return
	}

	var handler http.HandlerFunc
	switch req.Method {
	case http.MethodGet:
		const managedDevices = "/v1.0/deviceManagement/managedDevices"

		if req.URL.Path == managedDevices {
			handler = a.listManagedDevices
			break
		}
	}

	if handler == nil {
		a.replyError(w, 404, api.GraphErrorResource{
			Code:    "BadRequest",
			Message: fmt.Sprintf("Resource not found for %s %q", req.Method, req.URL.Path),
		})
		return
	}

	handler(w, req)
}

func (a *API) isAuthorized(req *http.Request) (*accessToken, bool) {
	authzHeader := req.Header.Get("Authorization")
	token, found := strings.CutPrefix(authzHeader, "Bearer ")
	if !found {
		return nil, false
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	issuedToken, ok := a.issuedTokens[token]
	if !ok {
		return nil, false
	}

	now := a.config.Clock.Now().UTC()
	if now.After(issuedToken.expires) {
		delete(a.issuedTokens, token)
		return nil, false
	}

	return issuedToken, true
}

type accessToken struct {
	owner   string
	token   string
	expires time.Time
}

const (
	// AccessTokenExpiryPeriod is the expiration period for the returned access token.
	// https://learn.microsoft.com/en-us/graph/auth-v2-service?tabs=http#token-response
	AccessTokenExpiryPeriod = 3599 * time.Second
)

func (a *API) postAccessToken(w http.ResponseWriter, req *http.Request, tenant string) {
	const invalidRequest = "invalid_request"

	// Require a specific Content-Type. This is a fake only check.
	if ct := req.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
		a.replyJSON(w, 400, api.LoginErrorResponse{
			Error:            invalidRequest,
			ErrorDescription: "wrong content type",
		})
		return
	}

	// Parse form from body. This is a fake only check.
	if err := req.ParseForm(); err != nil {
		a.replyJSON(w, 400, api.LoginErrorResponse{
			Error:            invalidRequest,
			ErrorDescription: "could not parse request body",
		})
		return
	}

	if grantType := req.PostForm.Get("grant_type"); grantType != "client_credentials" {
		a.replyJSON(w, 400, api.LoginErrorResponse{
			Error:            "unsupported_grant_type",
			ErrorDescription: "unsupported grant type",
		})
		return
	}

	if scope := req.PostForm.Get("scope"); scope != "https://graph.microsoft.com/.default" {
		a.replyJSON(w, 400, api.LoginErrorResponse{
			Error:            invalidRequest,
			ErrorDescription: "missing or invalid scope",
		})
		return
	}

	tenants := a.getTenants()
	if _, ok := tenants[tenant]; !ok {
		a.replyJSON(w, 400, api.LoginErrorResponse{
			// https://login.microsoftonline.com/error?code=90002
			Error:            invalidRequest,
			ErrorDescription: "tenant not found",
			ErrorCodes:       []int{api.DiagCodeTenantNotFound},
		})
		return
	}

	clientID := req.PostForm.Get("client_id")
	clientSecret := req.PostForm.Get("client_secret")

	// Find app credentials. Simulates the identity platform behavior which returns a different error
	// if the client ID is not found at all and if the client secret doesn't match.
	match := false
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, app := range a.apps {
		if app.Tenant == tenant && app.ClientID == clientID {
			if app.ClientSecret == clientSecret {
				match = true
				break
			}
			a.replyJSON(w, 401, api.LoginErrorResponse{
				// https://login.microsoftonline.com/error?code=7000215
				Error:            "invalid_client",
				ErrorDescription: "invalid client secret provided",
			})
			return
		}
	}
	if !match {
		a.replyJSON(w, 400, api.LoginErrorResponse{
			// https://login.microsoftonline.com/error?code=700016
			Error:            "unauthorized_client",
			ErrorDescription: "app not found",
		})
		return
	}

	if token := a.issueAuthTokenLocked(w, clientID); token != nil {
		a.replyJSON(w, 200, api.AccessToken{
			AccessToken: token.token,
			ExpiresIn:   int(AccessTokenExpiryPeriod.Seconds()),
		})
	}
}

func (a *API) issueAuthTokenLocked(w http.ResponseWriter, clientID string) *accessToken {
	token, err := a.newAuthToken(clientID)
	if err != nil {
		// Error not observed in practice.
		a.replyJSON(w, 500, api.LoginErrorResponse{
			Error:            "auth_token_error",
			ErrorDescription: err.Error(),
		})
		return nil
	}

	a.issuedTokens[token.token] = token

	return token
}

func (a *API) newAuthToken(clientID string) (*accessToken, error) {
	// An opaque string is good enough for our purposes.
	// Size is arbitrary.
	token := make([]byte, 40)
	if _, err := rand.Read(token); err != nil {
		return nil, fmt.Errorf("reading random bytes: %w", err)
	}

	tokenB64 := base64.StdEncoding.EncodeToString(token)
	expires := a.config.Clock.Now().Add(AccessTokenExpiryPeriod).UTC()
	return &accessToken{
		token:   tokenB64,
		expires: expires,
		owner:   clientID,
	}, nil
}

func (a *API) replyError(w http.ResponseWriter, code int, error api.GraphErrorResource) {
	a.replyJSON(w, code, api.GraphErrorResponse{Error: error})
}

func (a *API) replyJSON(w http.ResponseWriter, code int, resp any) {
	body, err := json.Marshal(resp)
	if err != nil {
		a.config.Logger.WarnContext(context.Background(),
			"Failed to marshal JSON response",
			"error", err,
		)
	}

	w.WriteHeader(code)
	w.Write(body)
}

func (a *API) listManagedDevices(w http.ResponseWriter, req *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()

	devices := a.managedDevices
	q := req.URL.Query()

	if rawFilter := q.Get("$filter"); rawFilter != "" {
		// In the fake API, this is the only supported filter.
		rawLastSync, found := strings.CutPrefix(rawFilter, "lastSyncDateTime gt ")
		if !found {
			a.replyError(w, 400, api.GraphErrorResource{Code: "BadRequest", Message: "Invalid $filter clause: unrecognized property"})
			return
		}
		lastSync, err := time.Parse(time.RFC3339, rawLastSync)
		if err != nil {
			a.replyError(w, 400, api.GraphErrorResource{
				Code: "BadRequest", Message: fmt.Sprintf("Invalid $filter clause: invalid time %s", rawLastSync)})
			return
		}

		devices = []*api.ManagedDevice{}
		for _, device := range a.managedDevices {
			if device.LastSyncDateTime.After(lastSync) {
				devices = append(devices, device)
			}
		}
	}

	var nextLink string
	if rawTop := q.Get("$top"); rawTop != "" && len(devices) > 0 {
		top, err := strconv.Atoi(rawTop)
		if err != nil || top < 1 {
			a.replyError(w, 400, api.GraphErrorResource{
				Code: "BadRequest", Message: fmt.Sprintf("Invalid $top value %q", rawTop)})
			return
		}
		// In the real API, $skipToken is an opaque token. In the fake API, it's the ID of the last
		// device that was returned from the previous page of results.
		lastDeviceID := q.Get("$skipToken")
		skippedDevicesCount := 0
		keepSkipping := lastDeviceID != ""
		var devicePage []*api.ManagedDevice

		for _, device := range devices {
			// If $skipToken is present, keep skipping devices until one is found that matches $skipToken.
			if keepSkipping {
				if device.ID == lastDeviceID {
					keepSkipping = false
				}
				skippedDevicesCount++
				continue
			}

			devicePage = append(devicePage, device)
			if len(devicePage) == top {
				break
			}
		}

		if keepSkipping {
			a.replyError(w, 400, api.GraphErrorResource{
				Code: "BadRequest", Message: "Could not find the device with the ID from $skipToken"})
			return
		}

		// Are there still more devices to return?
		if len(devices) > skippedDevicesCount+len(devicePage) {
			nextURL := *req.URL
			q := nextURL.Query()
			nextLastDeviceID := devicePage[len(devicePage)-1].ID
			q.Set("$skipToken", nextLastDeviceID)
			nextURL.Scheme = "https"
			nextURL.Host = req.Host
			nextURL.RawQuery = q.Encode()
			nextLink = nextURL.String()
		}
		devices = devicePage
	}

	a.replyJSON(w, 200, &api.ListManagedDevicesResponse{
		ManagedDevices: devices,
		NextLink:       nextLink,
	})
}
