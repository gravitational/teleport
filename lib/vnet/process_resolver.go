// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package vnet

import (
	"context"
	"log/slog"
)

type peerProcess struct {
	PID     int
	ExePath string
}

func (p peerProcess) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Int("pid", p.PID),
		slog.String("exe", p.ExePath),
	)
}

type processResolver interface {
	// resolveTCP returns the local process that owns the TCP connection whose
	// source (ephemeral) port is srcPort and destination port is dstPort.
	resolveTCP(srcPort, dstPort uint16) (peerProcess, error)
}

type peerProcessContextKey struct{}

// contextWithPeerProcess returns a copy of ctx carrying the local process that
// opened the connection being handled, so downstream connection callbacks can
// report it without re-resolving.
func contextWithPeerProcess(ctx context.Context, p peerProcess) context.Context {
	return context.WithValue(ctx, peerProcessContextKey{}, p)
}

// clientProcessPathFromContext returns the executable path of the local process
// carried by ctx, or an empty string if none was identified.
func clientProcessPathFromContext(ctx context.Context) string {
	p, ok := ctx.Value(peerProcessContextKey{}).(peerProcess)
	if !ok {
		return ""
	}
	return p.ExePath
}
