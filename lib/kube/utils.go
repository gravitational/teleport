package kube

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/authz"
)

const (
	// TeleportImpersonateUserHeader is a header that specifies teleport user identity
	// that the proxy is impersonating.
	TeleportImpersonateUserHeader = "Teleport-Impersonate-User"
	// TeleportImpersonateIPHeader is a header that specifies the real user IP address.
	TeleportImpersonateIPHeader = "Teleport-Impersonate-IP"
)

// ImpersonatorRoundTripper is a round tripper that impersonates a user with
// the identity provided.
type ImpersonatorRoundTripper struct {
	http.RoundTripper
}

// NewImpersonatorRoundTripper returns a new impersonator round tripper.
func NewImpersonatorRoundTripper(rt http.RoundTripper) *ImpersonatorRoundTripper {
	return &ImpersonatorRoundTripper{
		RoundTripper: rt,
	}
}

// RoundTrip implements http.RoundTripper interface to include the identity
// in the request header.
func (r *ImpersonatorRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())

	identity, err := authz.UserFromContext(req.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	b, err := json.Marshal(identity.GetIdentity())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.Header.Set(TeleportImpersonateUserHeader, string(b))

	clientSrcAddr, err := authz.ClientSrcAddrFromContext(req.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req.Header.Set(TeleportImpersonateIPHeader, clientSrcAddr.String())

	return r.RoundTripper.RoundTrip(req)
}

// CloseIdleConnections ensures that the returned [net.RoundTripper]
// has a CloseIdleConnections method.
func (r *ImpersonatorRoundTripper) CloseIdleConnections() {
	type closeIdler interface {
		CloseIdleConnections()
	}
	if c, ok := r.RoundTripper.(closeIdler); ok {
		c.CloseIdleConnections()
	}
}

// IdentityForwardingHeaders returns a copy of the provided headers with
// the TeleportImpersonateUserHeader and TeleportImpersonateIPHeader headers
// set to the identity provided.
// The returned headers shouln't be used across requests as they contain
// the client's IP address and the user's identity.
func IdentityForwardingHeaders(ctx context.Context, originalHeaders http.Header) (http.Header, error) {
	identity, err := authz.UserFromContext(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	b, err := json.Marshal(identity.GetIdentity())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	headers := originalHeaders.Clone()
	headers.Set(TeleportImpersonateUserHeader, string(b))

	clientSrcAddr, err := authz.ClientSrcAddrFromContext(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	headers.Set(TeleportImpersonateIPHeader, clientSrcAddr.String())
	return headers, nil
}
