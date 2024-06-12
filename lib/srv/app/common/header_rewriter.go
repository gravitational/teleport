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

package common

import (
	"net/http/httputil"

	"github.com/gravitational/teleport/lib/httplib/reverseproxy"
)

const (
	sslOn  = "on"
	sslOff = "off"
)

// HeaderRewriter delegates to rewriters and then appends its own headers.
type HeaderRewriter struct {
	delegates []reverseproxy.Rewriter
}

// NewHeaderRewriter will create a new header rewriter with a number of delegates.
// The delegates will be executed in the order supplied
func NewHeaderRewriter(delegates ...reverseproxy.Rewriter) *HeaderRewriter {
	return &HeaderRewriter{
		delegates: delegates,
	}
}

// Rewrite will delegate to the supplied delegates' rewrite functions and then inject
// its own headers.
func (hr *HeaderRewriter) Rewrite(req *httputil.ProxyRequest) {
	for _, delegate := range hr.delegates {
		delegate.Rewrite(req)
	}

	if req.Out.TLS != nil {
		req.Out.Header.Set(XForwardedSSL, sslOn)
	} else {
		req.Out.Header.Set(XForwardedSSL, sslOff)
	}
}
