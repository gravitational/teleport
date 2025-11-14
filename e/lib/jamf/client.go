package jamf

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log"
)

const (
	userAgentProduct = "teleport"
	// For example: "teleport/16.4.6" (version without the the "v").
	userAgent = userAgentProduct + "/" + api.Version
)

var (
	// ErrJamfClientInvalidCredential is returned by Jamf client when the Jamf API
	// credentials are invalid.
	ErrJamfClientInvalidCredential = errors.New("invalid Jamf API credentials")

	// ErrJamfClientInvalidPrivilege is returned by Jamf client when the Jamf API
	// permissions are invalid.
	ErrJamfClientInvalidPrivilege = errors.New("invalid Jamf API permissions, verify your role and permission setup")
)

type authToken interface {
	GetAccessToken() string
	GetExpires() time.Time
}

// Client is the Jamf API client.
// It automatically manages authentication and refreshes existing bearer tokens,
// as appropriate.
type Client struct {
	clock      clockwork.Clock
	logger     *slog.Logger
	httpClient *http.Client

	baseURL                 string
	useComputersInventoryV2 bool

	// userPass or clientSecret are set depending on which type of API credentials
	// are supplied during Client creation.
	userPass     *userPasswordCreds
	clientSecret *clientSecretCreds

	// mu guards the fields below it.
	mu                    sync.Mutex
	currentToken          authToken
	repeatedAuthnFailures int
}

// ClientOpts are the creation options for the [Client].
type ClientOpts struct {
	Clock  clockwork.Clock
	Logger *slog.Logger

	HTTPClient *http.Client

	// APIURL is the URL for the Jamf API, usually including the "/api" path.
	// Example: "https://yourtenant.jamfcloud.com/api".
	APIURL string

	// Username for the Jamf API.
	// Prefer using ClientID and ClientSecret.
	Username string
	// Password for the Jamf API.
	// Prefer using ClientID and ClientSecret.
	Password string

	// ClientID is the Jamf API client ID.
	// See https://developer.jamf.com/jamf-pro/docs/client-credentials.
	ClientID string
	// ClientSecret is the Jamf API client secret.
	// See https://developer.jamf.com/jamf-pro/docs/client-credentials.
	ClientSecret string
}

// NewClient creates a new Jamf API client.
// `ctx` is used to verify credentials against the Jamf API.
func NewClient(ctx context.Context, opts ClientOpts) (*Client, error) {
	switch {
	case opts.HTTPClient == nil:
		return nil, trace.BadParameter("param HTTPClient required")
	case opts.APIURL == "":
		return nil, trace.BadParameter("param APIURL required")
	}

	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Verify credentials, either user+pass or clientID+secret.
	hasUserPass := opts.Username != "" && opts.Password != ""
	hasAPICreds := opts.ClientID != "" && opts.ClientSecret != ""
	var userPass *userPasswordCreds
	var clientSecret *clientSecretCreds
	switch {
	case !hasUserPass && !hasAPICreds:
		return nil, trace.BadParameter("client credentials required, either set ClientID+ClientSecret (preferred) or Username+Password (legacy)")
	case hasUserPass && hasAPICreds:
		logger.InfoContext(ctx, "Both Username+Password and ClientID+ClientSecret are set, using the latter for authentication")
		fallthrough
	case hasAPICreds:
		clientSecret = &clientSecretCreds{
			clientID:     opts.ClientID,
			clientSecret: opts.ClientSecret,
		}
	default:
		userPass = &userPasswordCreds{
			username: opts.Username,
			password: opts.Password,
		}
	}

	u, err := url.Parse(opts.APIURL)
	switch {
	case err != nil:
		return nil, trace.Wrap(err)
	case u.Host == "":
		return nil, trace.BadParameter("jamf APIURL lacks host: %q", opts.APIURL)
	}
	baseURL := &url.URL{
		Scheme: "https",
		Host:   u.Host,
		Path:   strings.TrimSuffix(u.Path, "/"),
	}

	clock := opts.Clock
	if clock == nil {
		clock = clockwork.NewRealClock()
	}

	// Forbid HTTP downgrades or changing from the base host.
	// https://github.com/gravitational/teleport-private/issues/916.
	httpClient := &http.Client{
		Transport: opts.HTTPClient.Transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			switch {
			// No downgrades, no host/port changes.
			// (Note that req.URL.Host already captures the port.)
			case req.URL.Scheme != "https" || req.URL.Host != baseURL.Host:
				return http.ErrUseLastResponse
			case opts.HTTPClient.CheckRedirect != nil:
				return opts.HTTPClient.CheckRedirect(req, via)
			default:
				return nil
			}
		},
		Jar:     opts.HTTPClient.Jar,
		Timeout: opts.HTTPClient.Timeout,
	}

	c := &Client{
		clock:        clock,
		logger:       logger,
		httpClient:   httpClient,
		userPass:     userPass,
		clientSecret: clientSecret,
	}
	if err := c.bootstrapClient(ctx, baseURL.String()); err != nil {
		return nil, trace.Wrap(err)
	}
	return c, nil
}

// bootstrapClient tests the credentials and discovers the correct values for
// c.baseURL and c.useComputersInventoryV2.
func (c *Client) bootstrapClient(ctx context.Context, initialURL string) error {
	if err := c.bootstrapBaseURL(ctx, initialURL); err != nil {
		return trace.Wrap(err)
	}

	apiVersion, err := c.discoverComputersInventoryVersion(ctx)
	if err != nil {
		return trace.Wrap(err, "determining supported /computers-inventory API version")
	}
	c.useComputersInventoryV2 = apiVersion == 2
	c.logger.DebugContext(ctx,
		"Jamf API: Determined /computers-inventory API version",
		"api_version", apiVersion,
	)

	return nil
}

func (c *Client) bootstrapBaseURL(ctx context.Context, initialURL string) error {
	// Prefer the "/api" suffixed version. It's more likely to succeed.
	urls := make([]string, 0, 2)
	if !strings.HasSuffix(initialURL, "/api") {
		urls = append(urls, initialURL+"/api")
	}
	urls = append(urls, initialURL)

	// Attempt to acquire an access token.
	var lastErr error
	for _, url := range urls {
		c.baseURL = url
		if _, lastErr = c.createOrRenewCurrentToken(ctx); lastErr == nil {
			break
		}
	}
	if lastErr != nil {
		apiError := &APIError{}
		_ = errors.As(lastErr, &apiError)

		switch apiError.StatusCode {
		case http.StatusUnauthorized:
			return trace.Wrap(ErrJamfClientInvalidCredential)
		case http.StatusForbidden:
			return trace.Wrap(ErrJamfClientInvalidPrivilege)
		default:
			return trace.Wrap(lastErr, "connecting to Jamf API")
		}
	}

	c.logger.DebugContext(ctx,
		"Jamf API: Authentication successful",
		"url", c.baseURL,
	)
	return nil
}

func (c *Client) discoverComputersInventoryVersion(ctx context.Context) (apiVersion int, _ error) {
	req := &GetComputersInventoryRequest{
		Page:     0,
		PageSize: 1,
	}

	_, errV2 := c.getV2ComputersInventory(ctx, req)
	if errV2 == nil {
		return 2, nil // Success
	}

	// A 404 means the v2 endpoint is not supported. Any other error is
	// unexpected.
	apiError := &APIError{}
	_ = errors.As(errV2, &apiError)
	if apiError.StatusCode != http.StatusNotFound {
		return 0, trace.Wrap(errV2, "/v2/computers-inventory")
	}

	_, errV1 := c.getV1ComputersInventory(ctx, req)
	if errV1 == nil {
		return 1, nil // Success
	}

	return 0, trace.NewAggregate(
		fmt.Errorf("/v2/computers-inventory: %w", errV2),
		fmt.Errorf("/v1/computers-inventory: %w", errV1),
	)
}

func (c *Client) endpoint(path string) string {
	return c.baseURL + path
}

func (c *Client) nowUTC() time.Time {
	return c.clock.Now().UTC()
}

func (c *Client) doJSONRequest(req *http.Request, jsonResp any) error {
	req.Header.Set("Accept", "application/json")
	// https://developer.jamf.com/developer-guide/docs/application-header-best-practices
	req.Header.Set("User-Agent", userAgent)

	// Log all requests at trace level.
	{
		authz := req.Header.Get("Authorization")
		req.Header.Set("Authorization", "redacted")

		// Unescape the query for clearer logs.
		u := req.URL.String()
		if val, err := url.QueryUnescape(u); err == nil {
			u = val
		}

		c.logger.Log(req.Context(), log.TraceLevel,
			"Client: Executing HTTP request",
			"url", u,
			"header", req.Header,
		)
		req.Header.Set("Authorization", authz)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return trace.Wrap(err)
	}

	// Always drain the body, regardless of the status code.
	body, err := utils.ReadAtMost(resp.Body, teleport.MaxHTTPResponseSize)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := resp.Body.Close(); err != nil {
		c.logger.WarnContext(req.Context(),
			"Jamf API: Failed to close http.Response body",
			"error", err,
		)
	}

	if resp.StatusCode != http.StatusOK {
		c.logger.DebugContext(req.Context(),
			"Jamf API: API request failed",
			"body", string(body),
			"status", resp.StatusCode,
			"url", req.URL,
		)
		return &APIError{
			StatusCode: resp.StatusCode,
		}
	}

	return trace.Wrap(json.Unmarshal(body, jsonResp))
}
