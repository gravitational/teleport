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

package accessgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/gravitational/trace"

	accessgraph "github.com/gravitational/teleport/lib/accessgraph/apiclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/utils"
)

// accessGraphAPIPath is the proxy-side base path the proxy mounts the Access Graph API on.
const accessGraphAPIPath = "/v1/enterprise/accessgraph/"

// accessGraphAPITimeout timeout for Access Graph API calls.
const accessGraphAPITimeout = 30 * time.Second

// apiResponseError small wrapper to capture errors returned by in AG API responses,
// which have the form of [accessgraph.BadRequest]
type apiResponseError struct {
	StatusCode int
	Message    string
}

func (e *apiResponseError) Error() string {
	return fmt.Sprintf("API request failed with status %d: %s", e.StatusCode, e.Message)
}

// accessGraphResponse is the common interface for generated client response types
type accessGraphResponse interface {
	StatusCode() int
	Bytes() []byte
}

// doRequest is meant to wrap a generated client call directly:
//
//	resp, err := doRequest(client.GetFooWithResponse(ctx, ...))
//
// It propagates transport errors and turns any HTTP status >= 400 into
// a non-nil error with a best-effort message extracted from the response body.
func doRequest[T accessGraphResponse](resp T, err error) (T, error) {
	var zero T
	if err != nil {
		return zero, trace.Wrap(err, "request failed")
	}
	if err := checkResponse(resp.StatusCode(), resp.Bytes()); err != nil {
		return zero, err
	}
	return resp, nil
}

// checkResponse inspects the HTTP status code and body of an API response, returning
// a non-nil error if the status code indicates failure. It attempts to extract
// a best-effort message from the response body.
func checkResponse(statusCode int, body []byte) error {
	if statusCode < 400 {
		return nil
	}

	var apiErr apiResponseError
	if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Message != "" {
		apiErr.StatusCode = statusCode
		return &apiErr
	}

	// Fall back to trace.ReadError, which handles teleport's standard error envelope
	return trace.ReadError(statusCode, body)
}

// newAccessGraphClient returns a generated Access Graph API client wired
// to talk to proxyAddr over mTLS using keyRing's AG cert.
func newAccessGraphClient(ctx context.Context, proxyAddr string, keyRing *client.KeyRing) (*accessgraph.ClientWithResponses, error) {
	if proxyAddr == "" {
		return nil, trace.BadParameter("missing proxy address")
	}

	// Normalize and validate  proxy address before using it to construct the client and HTTP transport.
	addr, err := utils.ParseAddr(proxyAddr)
	if err != nil {
		return nil, trace.Wrap(err, "parsing proxy address %q", proxyAddr)
	}

	httpClient, err := newAccessGraphHTTPClient(ctx, addr.Addr, keyRing)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	baseURL := (&url.URL{Scheme: "https", Host: addr.Addr, Path: accessGraphAPIPath}).String()
	accessGraphClient, err := accessgraph.NewClientWithResponses(
		baseURL,
		accessgraph.WithHTTPClient(httpClient),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	slog.DebugContext(ctx, "Initialized Access Graph API client",
		"proxy_addr", addr.Addr,
		"username", keyRing.Username,
	)
	return accessGraphClient, nil
}

// newAccessGraphHTTPClient returns an http.Client whose transpor presents keyRing's AG cert as mTLS to proxyAddr.
func newAccessGraphHTTPClient(ctx context.Context, proxyAddr string, keyRing *client.KeyRing) (*http.Client, error) {
	if keyRing == nil {
		return nil, trace.BadParameter("missing key ring")
	}

	baseTLSConfig, err := keyRing.AccessGraphClientTLSConfig(nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	httpClient := &http.Client{
		// Honor HTTPS_PROXY / NO_PROXY, matching the webclient used by
		// `tsh login` against this address; mTLS is end-to-end via CONNECT.
		Transport: &http.Transport{
			TLSClientConfig: baseTLSConfig,
			Proxy:           http.ProxyFromEnvironment,
		},
		Timeout: accessGraphAPITimeout,
	}

	slog.DebugContext(ctx, "Created Access Graph HTTP client",
		"proxy_addr", proxyAddr,
		"server_name", baseTLSConfig.ServerName,
	)
	return httpClient, nil
}
