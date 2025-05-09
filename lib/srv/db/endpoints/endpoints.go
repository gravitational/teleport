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

package endpoints

import (
	"context"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/gcp"
)

var (
	resolverBuilders   = make(map[string]ResolverBuilder)
	resolverBuildersMu sync.RWMutex
)

// ResolverBuilder constructs a [Resolver].
type ResolverBuilder func(ctx context.Context, db types.Database, cfg ResolverBuilderConfig) (Resolver, error)

// Resolver resolves database endpoints.
type Resolver interface {
	// Resolve resolves database endpoints.
	Resolve(ctx context.Context) ([]string, error)
}

// ResolverFn is function that implements [Resolver].
type ResolverFn func(ctx context.Context) ([]string, error)

// Resolve resolves database endpoints.
func (f ResolverFn) Resolve(ctx context.Context) ([]string, error) {
	return f(ctx)
}

// ResolverBuilderConfig is the config a for a [ResolverBuilder].
type ResolverBuilderConfig struct {
	// GCPClients are clients used to resolve GCP endpoints.
	GCPClients GCPClients
}

// GCPClients are clients used to resolve GCP endpoints.
type GCPClients interface {
	// GetGCPSQLAdminClient returns GCP Cloud SQL Admin client.
	GetGCPSQLAdminClient(context.Context) (gcp.SQLAdminClient, error)
}

// RegisterResolver registers a new database endpoint resolver.
func RegisterResolver(builder ResolverBuilder, names ...string) {
	resolverBuildersMu.Lock()
	defer resolverBuildersMu.Unlock()
	for _, name := range names {
		resolverBuilders[name] = builder
	}
}

// GetResolverBuilders is used in tests to cleanup after overriding a resolver.
func GetResolverBuilders(names ...string) (map[string]ResolverBuilder, error) {
	resolverBuildersMu.RLock()
	defer resolverBuildersMu.RUnlock()
	out := map[string]ResolverBuilder{}
	for _, name := range names {
		builder, ok := resolverBuilders[name]
		if !ok {
			return nil, trace.NotFound("database endpoint resolver builder %q is not registered", name)
		}
		out[name] = builder
	}
	return out, nil
}

// GetResolver returns a resolver for the given database.
func GetResolver(ctx context.Context, db types.Database, cfg ResolverBuilderConfig) (Resolver, error) {
	name := db.GetProtocol()
	resolverBuildersMu.RLock()
	builder, ok := resolverBuilders[name]
	resolverBuildersMu.RUnlock()
	if !ok {
		return nil, trace.NotFound("database endpoint resolver %q is not registered", name)
	}

	resolver, err := builder(ctx, db, cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resolver, nil
}

// IsRegistered returns true if the given database protocol has been registered.
func IsRegistered(db types.Database) bool {
	name := db.GetProtocol()
	resolverBuildersMu.RLock()
	defer resolverBuildersMu.RUnlock()
	_, ok := resolverBuilders[name]
	return ok
}
