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
	"crypto/x509"
	"net/http"
	"net/url"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
)

func NewInsecureWebClient() *http.Client {
	tlsConfig := utils.TLSConfig()
	tlsConfig.InsecureSkipVerify = true

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}
}

func newClientWithPool(pool *x509.CertPool) *http.Client {
	tlsConfig := utils.TLSConfig()
	tlsConfig.RootCAs = pool

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
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

func (w *WebClient) PostJSON(
	endpoint string, val interface{}) (*roundtrip.Response, error) {
	return httplib.ConvertResponse(w.Client.PostJSON(endpoint, val))
}

func (w *WebClient) PutJSON(
	endpoint string, val interface{}) (*roundtrip.Response, error) {
	return httplib.ConvertResponse(w.Client.PutJSON(endpoint, val))
}

func (w *WebClient) Get(endpoint string, val url.Values) (*roundtrip.Response, error) {
	return httplib.ConvertResponse(w.Client.Get(endpoint, val))
}

func (w *WebClient) Delete(endpoint string) (*roundtrip.Response, error) {
	return httplib.ConvertResponse(w.Client.Delete(endpoint))
}
