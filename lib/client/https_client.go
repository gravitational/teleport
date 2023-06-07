/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

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

	"github.com/gravitational/teleport"
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
	clt, err := roundtrip.NewClient(url, teleport.WebAPIVersion, opts...)
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
	log.Debugf("Attempting %s", endpoint)
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
	log.Warnf("Request for %s/%s falling back to PLAIN HTTP", u.Host, u.Path)
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
