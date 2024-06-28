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
