// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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
