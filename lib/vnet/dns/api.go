package dns

import (
	"context"
)

type Resolver interface {
	ResolveA(ctx context.Context, domain string) (Result, error)
	ResolveAAAA(ctx context.Context, domain string) (Result, error)
}

// Result holds the result of DNS resolution.
type Result struct {
	// A is an A record.
	A [4]byte
	// AAAA is an AAAA record.
	AAAA [16]byte
	// NXDomain indicates that the requested domain is invalid or unassigned and
	// the answer is authoritative.
	NXDomain bool
	// NoRecord indicates the domain exists but the requested record type
	// doesn't.
	NoRecord bool
}
