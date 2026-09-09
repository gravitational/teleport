/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package genericoidc

import (
	"cmp"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gravitational/trace"
	"github.com/ohler55/ojg/jp"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

type envGetter func(key string) string

type commandRunner func(ctx context.Context, command ...string) ([]byte, error)

type httpRequester func(ctx context.Context, params JWTFromHTTPEndpointParams) ([]byte, error)

// IDTokenSource allows a generic OIDC token to be fetched whilst within a job
// execution.
type IDTokenSource struct {
	getEnv      envGetter
	runCommand  commandRunner
	httpRequest httpRequester
}

// GetIDTokenFromEnvironment fetches a JWT from the local node's environment
func (its *IDTokenSource) GetIDTokenFromEnvironment(key string) (string, error) {
	tok := its.getEnv(key)
	if tok == "" {
		return "", trace.BadParameter(
			"environment variable %q is missing, ensure it exists and contains a valid JWT for OIDC joining",
			key,
		)
	}

	return strings.TrimSpace(tok), nil
}

// GetIDTokenFromCommand executes a command to fetch a JWT. The command may take
// up to the given timeout to execute, and callers are recommended to provide a
// sensible default timeout (e.g. 1 minute) by default.
func (its *IDTokenSource) GetIDTokenFromCommand(ctx context.Context, timeout time.Duration, command ...string) (string, error) {
	if len(command) == 0 {
		return "", trace.BadParameter("at least one command argument is required")
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	bytes, err := its.runCommand(timeoutCtx, command...)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Do some minimal validation. We'd otherwise like to check that this is a
	// parseable JWT, but doing so isn't worth including JWT libraries in client
	// binaries.
	if !utf8.Valid(bytes) {
		return "", trace.BadParameter("generic_oidc: retrieved token with invalid content")
	}

	return strings.TrimSpace(string(bytes)), nil
}

// allowedHTTPHosts defines the set of HTTP hosts that are considered safe to fetch tokens from when using `http://`.
// `https://` requests are considered safe by default and do not need to be listed here.
// For other HTTP hosts, users can set insecure_allow_http to true in configuration and bypass this restriction.
var allowedHTTPHosts = []string{
	"169.254.169.254",
	"127.0.0.1",
}

// GetIDTokenFromHTTPEndpoint fetches a JWT from an HTTP endpoint.
func (its *IDTokenSource) GetIDTokenFromHTTPEndpoint(ctx context.Context, timeout time.Duration, params JWTFromHTTPEndpointParams) (string, error) {
	if !params.IsSet() {
		return "", trace.BadParameter("at least the URL must be provided to fetch a token from an HTTP endpoint")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	bs, err := its.httpRequest(ctx, params)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Do some minimal validation. We'd otherwise like to check that this is a
	// parseable JWT, but doing so isn't worth including JWT libraries in client
	// binaries.
	if !utf8.Valid(bs) {
		return "", trace.BadParameter("generic_oidc: retrieved token from http_request has invalid content")
	}

	return strings.TrimSpace(string(bs)), nil
}

func DefaultCommandRunner(ctx context.Context, command ...string) ([]byte, error) {
	executable := command[0]
	args := command[1:]

	dir, err := os.UserHomeDir()
	if err != nil {
		return nil, trace.Wrap(err, "determining home directory")
	}

	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = dir

	// Inherit stderr - if errors are printed, they should be visible to the
	// user. We could buffer them and wrap them in our own log, but that may
	// make debugging more difficult.
	cmd.Stderr = os.Stderr

	bytes, err := cmd.Output()
	if err != nil {
		return nil, trace.Wrap(err, "generic_oidc: failed to run command to fetch token, see previous output for details")
	}

	return bytes, nil
}

// JWTFromHTTPEndpointParams contains the params to obtain a JWT from an HTTP endpoint.
type JWTFromHTTPEndpointParams struct {
	// Request contains the params to make the HTTP request.
	Request HTTPRequestParams
	// Result describes how the JWT can be extracted from the response.
	Result ResponseExtractParams
}

// IsSet returns whether the required params are set.
func (p *JWTFromHTTPEndpointParams) IsSet() bool {
	// Everything else has defaults, so we only need to check if the URL is set to determine if this is configured.
	return p.Request.URL != ""
}

// HTTPRequestParams contains the params to make an HTTP request.
type HTTPRequestParams struct {
	// Method is the HTTP method to use.
	// Defaults to GET.
	Method string
	// URL to be requested.
	URL string
	// QueryParams are the query parameters to include in the request.
	// If specified, they will be added to the URL's query string, possibly overriding any existing query parameters.
	QueryParams map[string]string
	// Headers are the HTTP headers to include in the request.
	Headers map[string]string
	// InsecureAllowHTTP indicates whether HTTP (non-HTTPS) requests are allowed.
	InsecureAllowHTTP bool
}

// ResponseExtractParams describes how the JWT can be extracted from the response.
type ResponseExtractParams struct {
	// JSONPath is a JSONPath expression that can be used to extract the JWT from the response body.
	// Example:
	// Extracting the "jwt" from a body which contains the following: `{"access_token": "jwt", ... }` would require the JSONPath expression `$.access_token`.
	JSONPath string
}

func isEndpointAllowed(endpointURL *url.URL, insecureHTTPAllowed bool) error {
	if endpointURL.Scheme == "https" {
		return nil
	}

	if endpointURL.Scheme == "http" {
		if insecureHTTPAllowed {
			return nil
		}
		if slices.Contains(allowedHTTPHosts, endpointURL.Hostname()) {
			return nil
		}
		return trace.BadParameter("generic_oidc: insecure HTTP request to host %q is not allowed, set 'http_request.request.insecure_allow_http' to true to allow non-HTTPS requests", endpointURL.Host)
	}

	return trace.BadParameter("generic_oidc: unsupported URL scheme %q", endpointURL.Scheme)
}

// DefaultHTTPRequester obtains a JWT from an HTTP endpoint and optionally
// extracts it from a JSON response using JSONPath.
func DefaultHTTPRequester(ctx context.Context, params JWTFromHTTPEndpointParams) ([]byte, error) {
	endpointURL, err := url.Parse(params.Request.URL)
	if err != nil {
		return nil, trace.Wrap(err, "generic_oidc: invalid URL")
	}

	if err := isEndpointAllowed(endpointURL, params.Request.InsecureAllowHTTP); err != nil {
		return nil, trace.Wrap(err)
	}

	query := endpointURL.Query()
	for key, value := range params.Request.QueryParams {
		query.Set(key, value)
	}
	endpointURL.RawQuery = query.Encode()

	method := cmp.Or(strings.ToUpper(params.Request.Method), http.MethodGet)
	req, err := http.NewRequestWithContext(ctx, method, endpointURL.String(), nil)
	if err != nil {
		return nil, trace.Wrap(err, "generic_oidc: creating HTTP request to fetch token")
	}

	for key, value := range params.Request.Headers {
		req.Header.Set(key, value)
	}

	// Disabling the HTTP Proxy is used to ensure that the JWT issuer endpoint is contacted directly.
	// This prevents the possibility of a malicious proxy intercepting the request/response.
	httpClient, err := defaults.HTTPClient(defaults.DisableProxyFromEnvironment())
	if err != nil {
		return nil, trace.Wrap(err, "generic_oidc: failed to create HTTP client")
	}
	// Disabling redirects, similar to the HTTP Proxy disabling, is used to ensure that the JWT issuer endpoint is contacted directly.
	httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, trace.Wrap(err, "generic_oidc: failed to make HTTP request")
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, trace.Errorf("generic_oidc: invalid status (%s) received from HTTP endpoint, only 2xx is considered successful", resp.Status)
	}

	body, err := utils.ReadAtMost(resp.Body, teleport.MaxHTTPResponseSize)
	if err != nil {
		return nil, trace.Wrap(err, "generic_oidc: failed to read HTTP response body")
	}

	// If no JSONPath is specified, return the raw body as the token.
	if params.Result.JSONPath == "" {
		return body, nil
	}

	expr, err := jp.ParseString(params.Result.JSONPath)
	if err != nil {
		return nil, trace.Wrap(err, "generic_oidc: invalid result.json_path expression %q, example of valid expressions: `$.access_token`, `$.id_token`", params.Result.JSONPath)
	}

	var response any
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, trace.Wrap(err, "generic_oidc: failed to unmarshal HTTP response body as JSON")
	}

	results := expr.Get(response)
	if len(results) != 1 {
		return nil, trace.BadParameter(
			"generic_oidc: result.json_path expression must select exactly one value, selected %d",
			len(results),
		)
	}

	token, ok := results[0].(string)
	if !ok {
		return nil, trace.BadParameter(
			"generic_oidc: result.json_path expression must select a string, selected %T",
			results[0],
		)
	}

	return []byte(token), nil
}

// NewIDTokenSource creates a new generic token source with the given audience
// tag.
func NewIDTokenSource(getEnv envGetter, runCommand commandRunner, httpRequest httpRequester) *IDTokenSource {
	return &IDTokenSource{
		getEnv:      getEnv,
		runCommand:  runCommand,
		httpRequest: httpRequest,
	}
}
