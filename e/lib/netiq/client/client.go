package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

// Config specifies dependencies and parameters for instantiating Client.
type Config struct {
	// OAuthClientID is the OAuth Client ID used to authenticate to OSP(NetIQ authorization service).
	OAuthClientID string
	// OAuthClientSecret is the OAuth Client Secret used to authenticate to OSP(NetIQ authorization service).
	OAuthClientSecret string
	// OSPURL is the URL of the OSP(NetIQ authorization service).
	OSPURL string
	// APIURL is the URL of the IDMProv API.
	APIURL string
	// IdentityVaultUser is the user used to authenticate to the Identity Vault.
	IdentityVaultUser string
	// IdentityVaultPassword is the password used to authenticate to the Identity Vault.
	IdentityVaultPassword string
	// InsecureSkipVerify is a flag that determines whether to skip verification of the server's certificate chain and host name.
	InsecureSkipVerify bool
	// Clock is the clock used to determine the current time.
	Clock clockwork.Clock
}
type Client struct {
	cfg                 Config
	authenticationToken *utils.FnCache
	tokenEndpoint       string
	httpClient          *http.Client
}

// New creates a new Client using the given config.
func New(ctx context.Context, cfg Config) (*Client, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, trace.Wrap(err)
	}

	httpClient, err := newHTTPClient(cfg.InsecureSkipVerify)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create HTTP client")
	}

	c := &Client{
		cfg:        cfg,
		httpClient: httpClient,
	}

	// authenticate to OSP to get the default authentication token expiration time.
	tokenResponse, err := c.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c.authenticationToken, err = utils.NewFnCache(
		utils.FnCacheConfig{
			Context: ctx,
			// TTL is the time to live for cache entries.
			TTL:   time.Duration(tokenResponse.ExpiresIn) * time.Second / 2,
			Clock: cfg.Clock,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create authentication token cache")
	}

	return c, nil
}

func (c *Client) authenticate(ctx context.Context) (tokenResponse, error) {
	if c.tokenEndpoint == "" {
		if err := c.getTokenEndpoint(); err != nil {
			return tokenResponse{}, trace.Wrap(err)
		}
	}

	q := url.Values{}
	q.Add("grant_type", "password")
	q.Add("username", c.cfg.IdentityVaultUser)
	q.Add("password", c.cfg.IdentityVaultPassword)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenEndpoint, strings.NewReader(q.Encode()))
	if err != nil {
		return tokenResponse{}, trace.Wrap(err, "failed to create request")
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Set the client ID and client secret for basic authentication.
	req.SetBasicAuth(c.cfg.OAuthClientID, c.cfg.OAuthClientSecret)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return tokenResponse{}, trace.Wrap(err, "failed to perform request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return tokenResponse{}, handleErrEndpoint(resp)
	}

	var token tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return tokenResponse{}, trace.Wrap(err, "failed to decode response")
	}

	return token, nil
}

func (c *Client) getAuthenticationToken(ctx context.Context) (string, error) {
	token, err := utils.FnCacheGet(
		ctx,
		c.authenticationToken,
		"token",
		func(ctx context.Context) (tokenResponse, error) {
			return c.authenticate(ctx)
		},
	)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return "Bearer " + token.AccessToken, nil
}

// tokenResponse represents the response from the OSP(NetIQ authorization service) token endpoint
// when requesting an access token.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// getTokenEndpoint retrieves the token endpoint from the OSP(NetIQ authorization service).
func (c *Client) getTokenEndpoint() error {
	u, err := url.Parse(c.cfg.OSPURL)
	if err != nil {
		return trace.Wrap(err, "failed to parse OSP URL")
	}

	const openIDConfigPath = "/a/idm/auth/oauth2/.well-known/openid-configuration"
	u.Path = path.Join(u.Path, openIDConfigPath)

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return trace.Wrap(err, "failed to create request")
	}

	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return trace.Wrap(err, "failed to perform request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return handleErrEndpoint(resp)
	}

	type openIDConfig struct {
		TokenEndpoint string `json:"token_endpoint"`
	}

	var config openIDConfig
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return trace.Wrap(err, "failed to decode response")
	}

	c.tokenEndpoint = config.TokenEndpoint

	return nil
}

type requestOption func(req *requestOptions)

type requestOptions struct {
	method      string
	body        io.Reader
	queryParams url.Values
}

func withPostRequest(body []byte) requestOption {
	return func(req *requestOptions) {
		req.method = http.MethodPost

		req.body = io.NopCloser(bytes.NewBuffer(body))
	}
}

func withQueryParams(key, val string) requestOption {
	return func(req *requestOptions) {
		req.queryParams.Set(key, val)
	}
}

func (c *Client) createAuthenticatedRequest(ctx context.Context, urlBase string, urlPath string, opts ...requestOption) (*http.Request, error) {
	reqOpts := &requestOptions{
		method:      http.MethodGet,
		queryParams: url.Values{},
	}
	for _, opt := range opts {
		opt(reqOpts)
	}

	token, err := c.getAuthenticationToken(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	u, err := url.Parse(urlBase)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse URL")
	}

	u.Path = path.Join(u.Path, urlPath)

	const (
		sizeKey = "size"
		sizeVal = "100"
	)
	q := u.Query()
	q.Set(sizeKey, sizeVal)

	for key, val := range reqOpts.queryParams {
		q[key] = val
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, reqOpts.method, u.String(), reqOpts.body)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create request")
	}

	req.Header.Set("Authorization", token)

	// Set the content type to JSON for POST requests.
	if reqOpts.method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}

	return req, nil
}

// newHTTPClient creates a new HTTP client with the given insecureSkipVerify flag.
func newHTTPClient(insecureSkipVerify bool) (*http.Client, error) {
	transport, err := defaults.Transport()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{}
	}
	transport.TLSClientConfig.InsecureSkipVerify = insecureSkipVerify

	return &http.Client{
		Transport: transport,
	}, nil
}

func listResponse[T interface{ GetNextIndex() int }, V any](
	ctx context.Context,
	c *Client,
	basePath string,
	convertFunc func(T) ([]V, error),
	opts ...requestOption,
) ([]V, error) {
	var (
		offset = 1
		output []V
	)

	for {
		req, err := c.createAuthenticatedRequest(
			ctx,
			c.cfg.APIURL,
			basePath,
			append(opts,
				withQueryParams("nextIndex", strconv.Itoa(offset)),
			)...,
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, handleErrEndpoint(resp)
		}

		var rsp T
		if err := json.NewDecoder(resp.Body).Decode(&rsp); err != nil {
			return nil, trace.Wrap(err, "failed to decode response")
		}

		out, err := convertFunc(rsp)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		output = append(output, out...)

		if rsp.GetNextIndex() <= 0 {
			break
		}
		offset = rsp.GetNextIndex()
	}
	return output, nil
}

func validateConfig(c Config) error {
	if c.OAuthClientID == "" {
		return trace.BadParameter("OAuthClientID is required")
	}
	if c.OAuthClientSecret == "" {
		return trace.BadParameter("OAuthClientSecret is required")
	}
	if c.OSPURL == "" {
		return trace.BadParameter("OSPURL is required")
	}
	if c.APIURL == "" {
		return trace.BadParameter("APIURL is required")
	}
	if c.IdentityVaultUser == "" {
		return trace.BadParameter("IdentityVaultUser is required")
	}
	if c.IdentityVaultPassword == "" {
		return trace.BadParameter("IdentityVaultPassword is required")
	}
	if c.Clock == nil {
		return trace.BadParameter("Clock is required")
	}
	return nil
}

func handleErrEndpoint(resp *http.Response) *ErrorResponse {
	var payload ErrorPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		switch resp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return &ErrorResponse{
				StatusCode: resp.StatusCode,
			}
		default:
			return &ErrorResponse{
				StatusCode: resp.StatusCode,
				Err:        trace.Wrap(err),
			}
		}
	}
	return &ErrorResponse{
		StatusCode: resp.StatusCode,
		Payload:    &payload,
	}
}

type idPayloadRequest struct {
	ID string `json:"id"`
}

func newPayloadRequestBody(id string) ([]byte, error) {
	b := new(bytes.Buffer)
	err := json.NewEncoder(b).Encode(&idPayloadRequest{
		ID: id,
	})
	return b.Bytes(), trace.Wrap(err)
}

type dnPayloadRequest struct {
	DN string `json:"dn"`
}

func newDNPayloadRequestBody(dn string) ([]byte, error) {
	b := new(bytes.Buffer)
	err := json.NewEncoder(b).Encode(&dnPayloadRequest{
		DN: dn,
	})
	return b.Bytes(), trace.Wrap(err)
}
