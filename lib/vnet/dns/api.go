package dns

import (
	"context"

	"gvisor.dev/gvisor/pkg/tcpip"
)

type Resolver interface {
	Resolve(ctx context.Context, domain string) (*Result, error)
}

// Result holds the result of DNS resolution.
type Result struct {
	// IP is the IP address.
	IP tcpip.Address
	// NXDomain indicates that the requested domain is invalid or unassigned.
	NXDomain bool
	// ForwardTo indicates that the DNS request should be forwarded to this
	// address.
	ForwardTo tcpip.Address
}
