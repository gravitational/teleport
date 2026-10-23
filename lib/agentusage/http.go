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

package agentusage

import (
	"encoding/base64"
	"net/http"
	"strings"

	prehogv1a "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
)

// Header is the HTTP header field for base64-encoded AgentUsageEvidence.
const Header = "Teleport-Agent-Usage"

func matchUserAgent(userAgent string) []prehogv1a.AgentTool {
	var tools []prehogv1a.AgentTool
	for _, definition := range toolEvidenceDefinitions {
		for _, product := range definition.userAgentProducts {
			if hasUserAgentProduct(userAgent, product) {
				tools = append(tools, definition.tool)
				break
			}
		}
	}
	return tools
}

// hasUserAgentProduct reports true when the User-Agent contains the complete
// product name. The product can have a version after a slash.
func hasUserAgentProduct(userAgent, want string) bool {
	products := strings.FieldsFuncSeq(userAgent, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '(' || r == ')' || r == ';'
	})
	for product := range products {
		name, _, _ := strings.Cut(product, "/")
		if name == want {
			return true
		}
	}
	return false
}

func parseHeader(headerValue string) *prehogv1a.AgentUsageEvidence {
	if headerValue == "" || len(headerValue) > base64.StdEncoding.EncodedLen(maxEncodedSize) {
		return nil
	}
	encoded, err := base64.StdEncoding.DecodeString(headerValue)
	if err != nil {
		return nil
	}
	return parse(encoded)
}

// WithRequestEvidence reads evidence for the user-facing proxy. It replaces
// the user-agent tools in the Header evidence with the tools it matches in the
// User-Agent header.
func WithRequestEvidence(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if evidence := requestEvidence(r); evidence != nil {
			r = r.WithContext(ContextWithEvidence(r.Context(), evidence))
		}
		next.ServeHTTP(w, r)
	})
}

func requestEvidence(r *http.Request) *prehogv1a.AgentUsageEvidence {
	evidence := parseHeader(r.Header.Get(Header))
	tools := matchUserAgent(r.UserAgent())
	if evidence == nil {
		if len(tools) == 0 {
			return nil
		}
		evidence = &prehogv1a.AgentUsageEvidence{}
	}
	evidence.KnownAgentUserAgent = tools
	if isEmpty(evidence) {
		return nil
	}
	return evidence
}

// HTTPHandler reads evidence from the Header for servers behind the proxy.
func HTTPHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if evidence := parseHeader(r.Header.Get(Header)); evidence != nil {
			r = r.WithContext(ContextWithEvidence(r.Context(), evidence))
		}
		next.ServeHTTP(w, r)
	})
}

// EncodeHeader returns the Header value for the evidence.
func EncodeHeader(evidence *prehogv1a.AgentUsageEvidence) string {
	encoded := encode(evidence)
	if len(encoded) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(encoded)
}

type roundTripper struct {
	next http.RoundTripper
}

// NewRoundTripper wraps next to forward evidence from the request context in
// the Header.
func NewRoundTripper(next http.RoundTripper) http.RoundTripper {
	return roundTripper{next: next}
}

func (t roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if headerValue := EncodeHeader(FromContext(req.Context())); headerValue != "" {
		req = req.Clone(req.Context())
		req.Header.Set(Header, headerValue)
	}
	return t.next.RoundTrip(req)
}

func (t roundTripper) Unwrap() http.RoundTripper {
	return t.next
}

func (t roundTripper) CloseIdleConnections() {
	type closeIdler interface {
		CloseIdleConnections()
	}
	if tr, ok := t.next.(closeIdler); ok {
		tr.CloseIdleConnections()
	}
}
