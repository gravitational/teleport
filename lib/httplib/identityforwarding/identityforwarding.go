// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// Package identityforwarding sets the headers that let a Proxy forward the
// identity behind a request to the service it calls, which reads them with
// [authz.Middleware].
package identityforwarding

import (
	"context"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"net/http"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/tlsca"
)

// RoundTripper adds the user identity and client address in each request's
// context as headers, which [authz.Middleware] accepts only from a Proxy.
type RoundTripper struct {
	http.RoundTripper
}

// NewRoundTripper wraps rt to forward identities.
func NewRoundTripper(rt http.RoundTripper) *RoundTripper {
	return &RoundTripper{
		RoundTripper: rt,
	}
}

// RoundTrip implements [http.RoundTripper] to include the identity
// in the request header.
func (r *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())

	identity, err := authz.UserFromContext(req.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	b, err := marshalIdentity(identity.GetIdentity())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.Header.Set(authz.TeleportImpersonateUserHeader, string(b))

	clientSrcAddr, err := authz.ClientSrcAddrFromContext(req.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req.Header.Set(authz.TeleportImpersonateIPHeader, clientSrcAddr.String())

	return r.RoundTripper.RoundTrip(req)
}

// CloseIdleConnections ensures that the returned [http.RoundTripper]
// has a CloseIdleConnections method.
func (r *RoundTripper) CloseIdleConnections() {
	type closeIdler interface {
		CloseIdleConnections()
	}
	if c, ok := r.RoundTripper.(closeIdler); ok {
		c.CloseIdleConnections()
	}
}

// Headers returns a copy of the provided headers with the identity and client
// address in ctx set in the headers that [authz.Middleware] reads.
// The returned headers shouldn't be used across requests as they contain
// the client's IP address and the user's identity.
func Headers(ctx context.Context, originalHeaders http.Header) (http.Header, error) {
	identity, err := authz.UserFromContext(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	b, err := marshalIdentity(identity.GetIdentity())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	headers := originalHeaders.Clone()
	headers.Set(authz.TeleportImpersonateUserHeader, string(b))

	clientSrcAddr, err := authz.ClientSrcAddrFromContext(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	headers.Set(authz.TeleportImpersonateIPHeader, clientSrcAddr.String())
	return headers, nil
}

// marshalIdentity preserves json v1 identity wire format while
// omitting zero values to keep forwarding headers small.
func marshalIdentity(identity tlsca.Identity) ([]byte, error) {
	//nolint:staticcheck // TODO(kiosion): deprecated, to be removed in v21
	identity.AllowedResourceIDs = identity.LegacyAllowedResourceIDs()

	return jsonv2.Marshal(
		&identity,
		json.DefaultOptionsV1(),
		jsonv2.OmitZeroStructFields(true),
	)
}
