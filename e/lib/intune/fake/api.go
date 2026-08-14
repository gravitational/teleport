package fake

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/e/lib/intune/api"
	"github.com/gravitational/teleport/lib/msgraph"
)

// API holds the implementation details of an HTTP handler for the fake Intune API.
type API struct {
	config Config

	// mu guards all fields below it
	mu                    sync.Mutex
	apps                  []*api.AppCredentials
	unauthorizedClientIDs []string
	managedDevices        []*msgraph.ManagedDevice
	issuedTokens          map[string]*accessToken // key is [accessToken.token]
	simulatePagingGaps    bool
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
		managedDevices: []*msgraph.ManagedDevice{},
	}
}

// SetApps overrides app credentials that can be used to authenticate with the fake API.
func (a *API) SetApps(apps []*api.AppCredentials) {
	a.mu.Lock()
	a.apps = apps
	a.mu.Unlock()
}

// SetUnauthorizedClientIDs sets a blocklist for clients that should not be able to access the Graph
// API. This helps to simulate a situation where a registered app has valid credentials but doesn't
// have necessary permissions set.
func (a *API) SetUnauthorizedClientIDs(clientIDs []string) {
	a.mu.Lock()
	a.unauthorizedClientIDs = clientIDs
	a.mu.Unlock()
}

// SetManagedDevices copies provided devices and sets it as the inventory in the fake API.
func (a *API) SetManagedDevices(devices []*msgraph.ManagedDevice) {
	a.mu.Lock()
	a.managedDevices = make([]*msgraph.ManagedDevice, 0, len(devices))
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

// SetSimulatePagingGaps enables simulation of paging gaps.
// If set to true, listing devices on Intune will return incomplete pages on most requests.
// Useful to test undue device deletions during inventory syncs.
func (a *API) SetSimulatePagingGaps(b bool) {
	a.mu.Lock()
	a.simulatePagingGaps = b
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
	dump, err := httputil.DumpRequest(req, true /* body, for token requests */)
	if err != nil {
		a.config.Logger.DebugContext(req.Context(), "Could not dump request", "error", err)
	} else {
		a.config.Logger.DebugContext(req.Context(), "Incoming request", "req", dump)
	}

	// Handle access token endpoint.
	//
	// In the actual Intune API, token requests are sent to login.microsoftonline.com and API requests
	// are sent to graph.microsoft.com.
	// In the fake API however, all requests are sent to the same host.
	//
	// https://learn.microsoft.com/en-us/graph/auth-v2-service?tabs=http#token-request
	const oauthTokenSuffix = "/oauth2/v2.0/token"
	const openIDconfigSuffix = "/v2.0/.well-known/openid-configuration"
	const oauthAuthorizeSuffix = "/oauth2/v2.0/authorize"

	// This is the endpoint azidentity.ClientSecretCredential sends a request to if
	// DisableInstanceDiscovery is false (which is the default). The response is based on the response
	// from an actual endpoint that the Azure SDK sends a request to.
	// https://login.microsoftonline.com/common/discovery/instance?api-version=1.1&authorization_endpoint=https%3A%2F%2Flogin.microsoftonline.com%2Fexample.onmicrosoft.com%2Foauth2%2Fv2.0%2Fauthorize
	if strings.HasSuffix(req.URL.Path, "/common/discovery/instance") {
		authorizationEndpoint := req.URL.Query().Get("authorization_endpoint")
		tenantBaseURL := strings.TrimSuffix(authorizationEndpoint, oauthAuthorizeSuffix)
		a.replyJSON(w, 200, map[string]string{
			// This seems to be the only field needed by the SDK to proceed.
			"tenant_discovery_endpoint": tenantBaseURL + openIDconfigSuffix,
		})
		return
	}

	if tenantRaw, isOpenIDConfigPath := strings.CutSuffix(req.URL.Path, openIDconfigSuffix); isOpenIDConfigPath {
		tenant := strings.Trim(tenantRaw, "/")
		tenantBaseURL := "https://" + req.Host + "/" + tenant
		a.replyJSON(w, 200, map[string]string{
			// Based on a response from an actual endpoint that the Azure SDK sends a request to.
			// This is the minimal set of fields needed for the SDK to not complain about missing values.
			// https://login.microsoftonline.com/example.onmicrosoft.com/v2.0/.well-known/openid-configuration
			"token_endpoint":         tenantBaseURL + oauthTokenSuffix,
			"authorization_endpoint": tenantBaseURL + oauthAuthorizeSuffix,
			"issuer":                 tenantBaseURL + "/v2.0",
		})
		return
	}

	if tenantRaw, isOauthPath := strings.CutSuffix(req.URL.Path, oauthTokenSuffix); isOauthPath {
		tenant := strings.Trim(tenantRaw, "/")
		a.postAccessToken(w, req, tenant)
		return
	}

	_, ok := a.isAuthorized(req)
	if !ok {
		a.replyError(w, 401, graphErrorResource{
			Code:    "InvalidAuthenticationToken",
			Message: "invalid authentication token",
		})
		return
	}

	var handler http.HandlerFunc
	switch req.Method {
	case http.MethodGet:
		const managedDevices = "/v1.0/deviceManagement/managedDevices"
		const managedDevicesSlash = managedDevices + "/"

		if req.URL.Path == managedDevices || req.URL.Path == managedDevicesSlash {
			handler = a.listManagedDevices
			break
		}

		// id guaranteed to be non-empty by the previous conditional.
		if id, ok := strings.CutPrefix(req.URL.Path, managedDevicesSlash); ok {
			handler = a.getManagedDevice(id)
		}
	}

	if handler == nil {
		a.replyError(w, 400, graphErrorResource{
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

	if slices.Contains(a.unauthorizedClientIDs, issuedToken.owner) {
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

	if err := req.ParseForm(); err != nil {
		a.replyJSON(w, 400, loginErrorResponse{
			Error:            invalidRequest,
			ErrorDescription: "could not parse request body",
		})
		return
	}

	if grantType := req.PostForm.Get("grant_type"); grantType != "client_credentials" {
		a.replyJSON(w, 400, loginErrorResponse{
			Error:            "unsupported_grant_type",
			ErrorDescription: "unsupported grant type",
		})
		return
	}

	tenants := a.getTenants()
	if _, ok := tenants[tenant]; !ok {
		a.replyJSON(w, 400, loginErrorResponse{
			// https://login.microsoftonline.com/error?code=90002
			Error:            invalidRequest,
			ErrorDescription: "tenant not found",
			ErrorCodes:       []int{msgraph.DiagCodeTenantNotFound},
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
			a.replyJSON(w, 401, loginErrorResponse{
				// https://login.microsoftonline.com/error?code=7000215
				Error:            "invalid_client",
				ErrorDescription: "invalid client secret provided",
			})
			return
		}
	}
	if !match {
		a.replyJSON(w, 400, loginErrorResponse{
			// https://login.microsoftonline.com/error?code=700016
			Error:            "unauthorized_client",
			ErrorDescription: "app not found",
		})
		return
	}

	if token := a.issueAuthTokenLocked(w, clientID); token != nil {
		a.replyJSON(w, 200, map[string]any{
			"access_token": token.token,
			"expires_in":   int(AccessTokenExpiryPeriod.Seconds()),
		})
	}
}

func (a *API) issueAuthTokenLocked(w http.ResponseWriter, clientID string) *accessToken {
	token, err := a.newAuthToken(clientID)
	if err != nil {
		// Error not observed in practice.
		a.replyJSON(w, 500, loginErrorResponse{
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

func (a *API) replyError(w http.ResponseWriter, code int, error graphErrorResource) {
	a.replyJSON(w, code, graphErrorResponse{Error: error})
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
			a.replyError(w, 400, graphErrorResource{Code: "BadRequest", Message: "Invalid $filter clause: unrecognized property"})
			return
		}
		lastSync, err := time.Parse(time.RFC3339, rawLastSync)
		if err != nil {
			a.replyError(w, 400, graphErrorResource{
				Code: "BadRequest", Message: fmt.Sprintf("Invalid $filter clause: invalid time %s", rawLastSync)})
			return
		}

		devices = []*msgraph.ManagedDevice{}
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
			a.replyError(w, 400, graphErrorResource{
				Code: "BadRequest", Message: fmt.Sprintf("Invalid $top value %q", rawTop)})
			return
		}
		// In the real API, $skipToken is an opaque token. In the fake API, it's the ID of the last
		// device that was returned from the previous page of results.
		lastDeviceID := q.Get("$skipToken")
		skippedDevicesCount := 0
		keepSkipping := lastDeviceID != ""
		var devicePage []*msgraph.ManagedDevice

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
			a.replyError(w, 400, graphErrorResource{
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

	if a.simulatePagingGaps && len(devices) > 0 {
		devices = devices[1:]
	}

	a.replyJSON(w, 200, map[string]any{
		"value":           devices,
		"@odata.nextLink": nextLink,
	})
}

func (a *API) getManagedDevice(id string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		a.mu.Lock()
		defer a.mu.Unlock()

		idx := slices.IndexFunc(a.managedDevices, func(md *msgraph.ManagedDevice) bool {
			return md.ID == id
		})

		if idx < 0 {
			a.replyError(w, 404, graphErrorResource{
				Code:    "ResourceNotFound",
				Message: "Resource not found",
			})
			return
		}

		device := a.managedDevices[idx]
		a.replyJSON(w, 200, device)
	}
}

// loginErrorResponse is the JSON shape returned by requests sent to LoginEndpoint of [Config].
// https://learn.microsoft.com/en-us/entra/identity-platform/reference-error-codes
type loginErrorResponse struct {
	// Error is the code string for the error, e.g. "invalid_client".
	Error string `json:"error"`
	// ErrorDescription is the detailed description of the error.
	ErrorDescription string `json:"error_description"`
	// ErrorCodes are codes returned by the security token service which map to specific reasons
	// as to why a request have failed.
	// https://learn.microsoft.com/en-us/entra/identity-platform/reference-error-codes#aadsts-error-codes
	ErrorCodes []int `json:"error_codes"`
}

// graphErrorResponse is the JSON shape returned by requests sent to GraphEndpoint of [Config].
// https://learn.microsoft.com/en-us/graph/errors#json-representation
type graphErrorResponse struct {
	Error graphErrorResource `json:"error"`
}

// graphErrorResource is the nested error resource within [graphErrorResponse].
// https://learn.microsoft.com/en-us/graph/errors#json-representation
type graphErrorResource struct {
	// Code is the code for the error, e.g. "UnknownError", "BadRequest".
	Code string `json:"code"`
	// Message is the detailed description of the error.
	Message string `json:"message"`
}
