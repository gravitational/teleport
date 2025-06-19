/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package client

import (
	"context"
	"log/slog"

	grpcprom "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/tbot/config"
)

// NewBuilder creates a new Builder.
func NewBuilder(cfg BuilderConfig) (*Builder, error) {
	return &Builder{cfg: cfg}, nil
}

// BuilderConfig contains the configuration options for a Builder.
type BuilderConfig struct {
	// Address that will be dialed to create the client connection.
	Address Address

	// AuthServerAddressMode controls the behavior when a proxy address is
	// given as an auth server address.
	AuthServerAddressMode config.AuthServerAddressMode

	// Resolver that will be used to find the address of a proxy server.
	Resolver reversetunnelclient.Resolver

	// Logger to which log messages will be written.
	Logger *slog.Logger

	// Insecure controls whether we will skip TLS host verification.
	Insecure bool

	// Metrics will record gRPC client metrics.
	Metrics *grpcprom.ClientMetrics
}

// Builder provides a convenient way to create a client for a given identity
// such as when services need to impersonate the a role to fetch a resource.
type Builder struct{ cfg BuilderConfig }

// Build a client for the given identity.
func (b *Builder) Build(ctx context.Context, id Identity) (*client.Client, error) {
	return New(ctx, Config{
		Identity: id,

		Address:               b.cfg.Address,
		AuthServerAddressMode: b.cfg.AuthServerAddressMode,
		Resolver:              b.cfg.Resolver,
		Logger:                b.cfg.Logger,
		Insecure:              b.cfg.Insecure,
		Metrics:               b.cfg.Metrics,
	})
}
