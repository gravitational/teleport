/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package client

import (
	"context"
	"crypto/x509"
	"net/http"
	"net/url"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"golang.org/x/net/http/httpproxy"

	tracehttp "github.com/gravitational/teleport/api/observability/tracing/http"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/utils"
)

func NewInsecureWebClient() *http.Client {
	return newClient(true, nil, nil)
}

func newClient(insecure bool, pool *x509.CertPool, extraHeaders map[string]string) *http.Client {
	return &http.Client{
		Transport: tracehttp.NewTransport(apiutils.NewHTTPRoundTripper(httpTransport(insecure, pool), extraHeaders)),
	}
}

func httpTransport(insecure bool, pool *x509.CertPool) *http.Transport {
	// Because Teleport clients can't be configured (yet), they take the default
	// list of cipher suites from Go.
	tlsConfig := utils.TLSConfig(nil)
	tlsConfig.InsecureSkipVerify = insecure
	tlsConfig.RootCAs = pool

	return &http.Transport{
		TLSClientConfig: tlsConfig,
		Proxy: func(req *http.Request) (*url.URL, error) {
			return httpproxy.FromEnvironment().ProxyFunc()(req.URL)
		},
	}
}

func NewWebClient(url string, opts ...roundtrip.ClientParam) (*WebClient, error) {
	opts = append(opts, roundtrip.SanitizerEnabled(true))
	// We do not add the version prefix since web api endpoints will contain
	// differing version prefixes.
	clt, err := roundtrip.NewClient(url, "" /* version prefix */, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &WebClient{clt}, nil
}

// WebClient is a package local lightweight client used
// in tests and some functions to handle errors properly
type WebClient struct {
	*roundtrip.Client
}

// PostJSONWithFallback serializes an object to JSON and attempts to execute a POST
// using HTTPS, and then fall back to plain HTTP under certain, very specific circumstances.
//   - The caller must specifically allow it via the allowHTTPFallback parameter, and
//   - The target host must resolve to the loopback address.
//
// If these conditions are not met, then the plain-HTTP fallback is not allowed,
// and a the HTTPS failure will be considered final.
func (w *WebClient) PostJSONWithFallback(ctx context.Context, endpoint string, val interface{}, allowHTTPFallback bool) (*roundtrip.Response, error) {
	// First try HTTPS and see how that goes
	log.DebugContext(ctx, "Attempting request", "endpoint", endpoint)
	resp, httpsErr := w.Client.PostJSON(ctx, endpoint, val)
	if httpsErr == nil {
		// If all went well, then we don't need to do anything else - just return
		// that response
		return httplib.ConvertResponse(resp, httpsErr)
	}

	// If we're not allowed to try plain HTTP, bail out with whatever error we have.
	if !allowHTTPFallback {
		return nil, trace.Wrap(httpsErr)
	}

	// Parse out the endpoint into its constituent parts. We will need the
	// hostname to decide if we're allowed to fall back to HTTPS, and we will
	// re-use this for re-writing the endpoint URL later on anyway.
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If we're allowed to try plain HTTP, but we're not on the loopback address,
	// bail out with whatever error we have.
	// Note that we're only allowed to try plain HTTP on the loopback address, even
	// if the caller says its OK.
	if !apiutils.IsLoopback(u.Host) {
		return nil, trace.Wrap(httpsErr)
	}

	// re-write the endpoint to try HTTP
	u.Scheme = "http"
	endpoint = u.String()
	log.WarnContext(ctx, "Request for falling back to PLAIN HTTP", "endpoint", endpoint)
	return httplib.ConvertResponse(w.Client.PostJSON(ctx, endpoint, val))
}

func (w *WebClient) PostJSON(ctx context.Context, endpoint string, val interface{}) (*roundtrip.Response, error) {
	return httplib.ConvertResponse(w.Client.PostJSON(ctx, endpoint, val))
}

func (w *WebClient) PutJSON(ctx context.Context, endpoint string, val interface{}) (*roundtrip.Response, error) {
	return httplib.ConvertResponse(w.Client.PutJSON(ctx, endpoint, val))
}

func (w *WebClient) Get(ctx context.Context, endpoint string, val url.Values) (*roundtrip.Response, error) {
	return httplib.ConvertResponse(w.Client.Get(ctx, endpoint, val))
}

func (w *WebClient) Delete(ctx context.Context, endpoint string) (*roundtrip.Response, error) {
	return httplib.ConvertResponse(w.Client.Delete(ctx, endpoint))
}

func (w *WebClient) DeleteWithParams(ctx context.Context, endpoint string, val url.Values) (*roundtrip.Response, error) {
	return httplib.ConvertResponse(w.Client.DeleteWithParams(ctx, endpoint, val))
}
