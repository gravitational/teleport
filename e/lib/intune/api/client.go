package api

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log"
)

// Client represents a client for the Intune API. It authorizes with the API through OAuth (see
// [AppCredentials]). It automatically exchanges credentials for a fresh access token when needed.
type Client struct {
	config ClientConfig
	// loginURL points to the login API. It is a parsed version of LoginEndpoint of [ClientConfig].
	loginURL *url.URL
	// graphURL points to the Microsoft Graph API. A parsed version of GraphEndpoint of [ClientConfig].
	graphURL *url.URL

	// mu protects repeatedAuthnFailures and currentToken.
	mu                    sync.Mutex
	repeatedAuthnFailures int
	currentToken          *AccessToken
}

// ClientConfig is the config used by [Client].
type ClientConfig struct {
	APIConfig  Config
	Logger     *slog.Logger
	Clock      clockwork.Clock
	HTTPClient *http.Client
}

// Config are parameters required by the Intune API itself.
type Config struct {
	// AppCredentials are credentials used to authenticate with the API.
	AppCredentials AppCredentials
	// LoginEndpoint points to one of the national deployments of Microsoft Entra ID.
	// Optional, defaults to "https://login.microsoftonline.com".
	//
	// https://learn.microsoft.com/en-us/graph/deployments
	LoginEndpoint string
	// GraphEndpoint points to one of the national deployments of Microsoft Graph.
	// Optional, defaults to "https://graph.microsoft.com".
	//
	// https://learn.microsoft.com/en-us/graph/deployments
	GraphEndpoint string
}

func NewClient(ctx context.Context, config ClientConfig) (*Client, error) {
	if err := ValidateAppCredentials(config.APIConfig.AppCredentials); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := types.ValidateMSGraphEndpoints(config.APIConfig.LoginEndpoint, config.APIConfig.GraphEndpoint); err != nil {
		return nil, trace.Wrap(err)
	}

	if config.Logger == nil {
		return nil, trace.BadParameter("param Logger required")
	}

	config.Clock = cmp.Or(config.Clock, clockwork.NewRealClock())
	config.APIConfig.LoginEndpoint = cmp.Or(config.APIConfig.LoginEndpoint, types.MSGraphDefaultLoginEndpoint)
	config.APIConfig.GraphEndpoint = cmp.Or(config.APIConfig.GraphEndpoint, types.MSGraphDefaultEndpoint)
	loginURL, err := url.Parse(config.APIConfig.LoginEndpoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	graphURL, err := url.Parse(config.APIConfig.GraphEndpoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{
			Timeout: 1 * time.Minute,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}

	c := &Client{
		config:   config,
		loginURL: loginURL,
		graphURL: graphURL,
	}
	if err := c.verifyCredentials(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return c, nil
}

var (
	// ErrIntuneClientTenantNotFound is returned by the Intune client when authorization fails
	// during the init of the client due to the tenant not being found. It might also point to the
	// subscription no longer being active.
	ErrIntuneClientTenantNotFound = errors.New("tenant not found")
	// ErrIntuneClientInvalidCredentials is returned by the Intune client when authorization fails
	// during the init of the client due to invalid client ID or secret.
	ErrIntuneClientInvalidCredentials = errors.New("invalid Intune API credentials")
)

const (
	// DiagCodeTenantNotFound is returned by the identity platform when the specific tenant doesn't
	// exist. It might also mean that the tenant belongs to another cloud (see
	// https://learn.microsoft.com/en-us/graph/deployments) or there are no active subscriptions for
	// the tenant.
	// https://login.microsoftonline.com/error?code=90002
	DiagCodeTenantNotFound = 90002
	// DiagCodeInvalidTenantIdentifier is returned by the identity platform when the identifier is
	// neither a valid DNS name nor a valid external domain. This happes when the tenant is not a UUID
	// and instead a regular string that doesn't match said requirements.
	// https://login.microsoftonline.com/error?code=900023
	DiagCodeInvalidTenantIdentifier = 900023
)

func (c *Client) verifyCredentials(ctx context.Context) error {
	_, err := c.ListManagedDevices(ctx, &ListManagedDevicesRequest{Top: 1})
	if err == nil {
		c.config.Logger.DebugContext(ctx, "Authentication successful", "url", c.loginURL.String())
		return nil
	}

	apiError := &Error{}
	// Returned error ignored on purpose, makes no difference in the logic below.
	_ = errors.As(err, &apiError)

	switch {
	case apiError.ServiceKind == ServiceKindLogin &&
		(slices.Contains(apiError.DiagnosticCodes, DiagCodeTenantNotFound) ||
			slices.Contains(apiError.DiagnosticCodes, DiagCodeInvalidTenantIdentifier)):
		return trace.Wrap(ErrIntuneClientTenantNotFound, apiError.Message)

	case apiError.ServiceKind == ServiceKindLogin &&
		apiError.StatusCode == http.StatusBadRequest &&
		apiError.Code == "unauthorized_client":
		// It likely means that the provided client ID doesn't exist under this tenant.
		// https://login.microsoftonline.com/error?code=700016
		return trace.Wrap(ErrIntuneClientInvalidCredentials, apiError.Message)

	case apiError.ServiceKind == ServiceKindLogin &&
		apiError.StatusCode == http.StatusUnauthorized:
		// It likely means that the provided client secret doesn't match the client ID.
		// https://login.microsoftonline.com/error?code=7000215
		return trace.Wrap(ErrIntuneClientInvalidCredentials, apiError.Message)

	default:
		return trace.Wrap(err, "verifying Intune credentials")
	}
}

func (c *Client) doRequest(req *http.Request, jsonResp any) error {
	requestID := uuid.NewString()
	req.Header.Set("Accept", "application/json")
	// https://learn.microsoft.com/en-us/graph/best-practices-concept#reliability-and-support
	req.Header.Set("client-request-id", requestID)

	{
		authz := req.Header.Get("Authorization")
		if authz != "" {
			req.Header.Set("Authorization", "redacted")
		}

		// Unescape the query for clearer logs.
		u := req.URL.String()
		if val, err := url.QueryUnescape(u); err == nil {
			u = val
		}

		c.config.Logger.Log(req.Context(), log.TraceLevel,
			"Client: Executing HTTP request",
			"url", u,
			"header", req.Header,
			"clock_time", c.config.Clock.Now(),
		)
		req.Header.Set("Authorization", authz)
	}

	resp, err := c.config.HTTPClient.Do(req)
	if err != nil {
		return trace.Wrap(err)
	}

	// Always drain the body, regardless of the status code.
	body, err := utils.ReadAtMost(resp.Body, teleport.MaxHTTPResponseSize)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := resp.Body.Close(); err != nil {
		c.config.Logger.WarnContext(req.Context(), "Failed to close http.Response body", "error", err)
	}

	if resp.StatusCode != http.StatusOK {
		c.config.Logger.DebugContext(req.Context(),
			"API request failed",
			"body", string(body),
			"status", resp.StatusCode,
			"url", req.URL,
			"client_request_id", requestID,
		)
		apiError := c.unmarshalAPIError(body, req)
		apiError.StatusCode = resp.StatusCode

		return apiError
	}

	return trace.Wrap(json.Unmarshal(body, jsonResp))
}

// unmarshalAPIError attempts to unmarshal the error response and use it to populate the fields of
// Error. ServiceKind is the only field that is guaranteed to be populated. The rest might be empty
// if unmarshaling failed.
func (c *Client) unmarshalAPIError(body []byte, req *http.Request) *Error {
	var error Error
	// Requests to the login API have no Authorization header set. We cannot depend on the host of the
	// request, because in tests the host is the same for both c.loginURL and c.graphURL.
	if req.Header.Get("Authorization") == "" {
		error.ServiceKind = ServiceKindLogin

		resp := &LoginErrorResponse{}
		if err := json.Unmarshal(body, resp); err != nil {
			c.config.Logger.DebugContext(req.Context(), "Could not unmarshal login error response", "error", err)
			return &error
		}

		error.Code = resp.Error
		error.Message = resp.ErrorDescription
		error.DiagnosticCodes = resp.ErrorCodes
	} else {
		error.ServiceKind = ServiceKindGraph

		resp := &GraphErrorResponse{}
		if err := json.Unmarshal(body, resp); err != nil {
			c.config.Logger.DebugContext(req.Context(), "Could not unmarshal Graph error response", "error", err)
			return &error
		}

		error.Code = resp.Error.Code
		error.Message = resp.Error.Message
	}

	return &error
}

// LoginErrorResponse is the JSON shape returned by requests sent to LoginEndpoint of [Config].
// https://learn.microsoft.com/en-us/entra/identity-platform/reference-error-codes
type LoginErrorResponse struct {
	// Error is the code string for the error, e.g. "invalid_client".
	Error string `json:"error"`
	// ErrorDescription is the detailed description of the error.
	ErrorDescription string `json:"error_description"`
	// ErrorCodes are codes returned by the security token service which map to specific reasons
	// as to why a request have failed.
	// https://learn.microsoft.com/en-us/entra/identity-platform/reference-error-codes#aadsts-error-codes
	ErrorCodes []int `json:"error_codes"`
}

// GraphErrorResponse is the JSON shape returned by requests sent to GraphEndpoint of [Config].
// https://learn.microsoft.com/en-us/graph/errors#json-representation
type GraphErrorResponse struct {
	Error GraphErrorResource `json:"error"`
}

// GraphErrorResource is the nested error resource within [GraphErrorResponse].
// https://learn.microsoft.com/en-us/graph/errors#json-representation
type GraphErrorResource struct {
	// Code is the code for the error, e.g. "UnknownError", "BadRequest".
	Code string `json:"code"`
	// Message is the detailsed description of the error.
	Message string `json:"message"`
}

// Error is an error returned by the Intune API.
type Error struct {
	// StatusCode is the status of the HTTP response. Guaranteed to be present.
	// https://learn.microsoft.com/en-us/graph/errors#http-status-codes
	StatusCode int
	// Code is the machine-readable code for the error, e.g. "BadRequest" (for Graph requests),
	// "invalid_client" (for login requests). It might be missing if there was a problem with
	// parsing the error response payload.
	// For login codes, see https://learn.microsoft.com/en-us/entra/identity-platform/reference-error-codes.
	// Graph APIs doesn't have a list of specific error codes, they instead depend on StatusCode.
	Code string
	// Message is the human-readable description of the error. It might be missing if there was a
	// problem with parsing the error response payload.
	Message string
	// DiagnosticCodes are codes returned by the security token service which map to specific reasons
	// as to why a request have failed. Non-empty only if ServiceKind is [ServiceKindLogin].
	// https://learn.microsoft.com/en-us/entra/identity-platform/reference-error-codes
	DiagnosticCodes []int
	// ServiceKind specifies which API returned the error.
	ServiceKind ServiceKind
}

// ServiceKind describes which API was contacted.
type ServiceKind int

const (
	ServiceKindUnspecified ServiceKind = iota
	// ServiceKindGraph represents requests to the Microsoft Graph API.
	ServiceKindGraph
	// ServiceKindLogin represents requests to the Microsoft identity platform API.
	ServiceKindLogin
)

// Error returns a textual representation of the error.
func (e *Error) Error() string {
	if e == nil {
		return "nil error"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "service_kind=%d status=%v code=%s", e.ServiceKind, e.StatusCode, e.Code)

	// e.DiagnosticCodes is always empty if e.ServiceKind is ServiceKindGraph.
	if len(e.DiagnosticCodes) > 0 {
		fmt.Fprintf(&b, " diag_codes=%v", e.DiagnosticCodes)
	}

	fmt.Fprintf(&b, " message=%s", e.Message)
	return b.String()
}
