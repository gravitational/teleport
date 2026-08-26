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

	"github.com/gravitational/trace"
	"github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
)

// NewBuilder creates a new Builder.
func NewBuilder(cfg BuilderConfig) (*Builder, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Builder{cfg: cfg}, nil
}

// BuilderConfig contains the configuration options for a Builder.
type BuilderConfig struct {
	// Connection contains the address etc. used to dial a connection to the
	// auth server.
	Connection connection.Config

	// Resolver that will be used to find the address of a proxy server.
	Resolver reversetunnelclient.Resolver

	// Logger to which log messages will be written.
	Logger *slog.Logger

	// Metrics will record gRPC client metrics.
	Metrics *prometheus.ClientMetrics
}

func (cfg *BuilderConfig) CheckAndSetDefaults() error {
	if err := cfg.Connection.Validate(); err != nil {
		return trace.Wrap(err)
	}
	if cfg.Resolver == nil {
		return trace.BadParameter("Resolver is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return nil
}

// Builder provides a convenient way to create a client for a given identity
// such as when services need to impersonate the a role to fetch a resource.
type Builder struct{ cfg BuilderConfig }

// Build a client for the given identity.
func (b *Builder) Build(ctx context.Context, id Identity) (*client.Client, error) {
	return New(ctx, Config{
		Identity:   id,
		Connection: b.cfg.Connection,
		Resolver:   b.cfg.Resolver,
		Logger:     b.cfg.Logger,
		Metrics:    b.cfg.Metrics,
	})
}
