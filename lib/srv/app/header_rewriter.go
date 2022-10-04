/*
Copyright 2022 Gravitational, Inc.

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

package app

import (
	"net/http"

	"github.com/gravitational/oxy/forward"
	"github.com/gravitational/teleport/lib/srv/app/common"
)

const (
	sslOn  = "on"
	sslOff = "off"
)

// headerRewriter delegates to oxy's rewriter and then appends its own headers.
type headerRewriter struct {
	delegate *forward.HeaderRewriter
}

// Rewrite will delegate to forward.HeaderRewriter's rewrite function and then inject
// its own headers.
func (hr *headerRewriter) Rewrite(req *http.Request) {
	hr.delegate.Rewrite(req)

	if req.TLS != nil {
		req.Header.Set(common.XForwardedSSL, sslOn)
	} else {
		req.Header.Set(common.XForwardedSSL, sslOff)
	}

	// Guess some default ports if the port is not explicitly set in the request.
	port := "80"
	if req.Header.Get(forward.XForwardedProto) == "https" {
		port = "443"
	}

	if req.URL.Port() != "" {
		req.Header.Set(common.XForwardedPort, req.URL.Port())
	} else {
		req.Header.Set(common.XForwardedPort, port)
	}
}
