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

package healthchecks

import (
	"context"
	"log/slog"
	"sync"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/healthcheck"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

var (
	healthCheckerBuilders   = make(map[string]HealthCheckerBuilder)
	healthCheckerBuildersMu sync.RWMutex
)

// RegisterHealthChecker registers a new database health checker.
func RegisterHealthChecker(builder HealthCheckerBuilder, names ...string) {
	healthCheckerBuildersMu.Lock()
	defer healthCheckerBuildersMu.Unlock()
	for _, name := range names {
		healthCheckerBuilders[name] = builder
	}
}

// GetHealthCheckBuilders is used in tests to cleanup after overriding a resolver.
func GetHealthCheckBuilders(names ...string) (map[string]HealthCheckerBuilder, error) {
	healthCheckerBuildersMu.RLock()
	defer healthCheckerBuildersMu.RUnlock()
	out := map[string]HealthCheckerBuilder{}
	for _, name := range names {
		builder, ok := healthCheckerBuilders[name]
		if !ok {
			return nil, trace.NotFound("database endpoint resolver builder %q is not registered", name)
		}
		out[name] = builder
	}
	return out, nil
}

// GetHealthChecker returns a health checker for the given database.
func GetHealthChecker(ctx context.Context, cfg HealthCheckerConfig) (healthcheck.HealthChecker, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	name := cfg.Database.GetProtocol()
	healthCheckerBuildersMu.RLock()
	builder, ok := healthCheckerBuilders[name]
	healthCheckerBuildersMu.RUnlock()
	if !ok {
		return nil, trace.NotFound("database endpoint resolver %q is not registered", name)
	}

	checker, err := builder(ctx, cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return checker, nil
}

// IsRegistered returns true if the given database protocol has been registered.
func IsRegistered(db types.Database) bool {
	name := db.GetProtocol()
	healthCheckerBuildersMu.RLock()
	defer healthCheckerBuildersMu.RUnlock()
	_, ok := healthCheckerBuilders[name]
	return ok
}

// HealthCheckerBuilder builds a database [healthcheck.HealthChecker].
type HealthCheckerBuilder func(ctx context.Context, cfg HealthCheckerConfig) (healthcheck.HealthChecker, error)

// HealthCheckerConfig is the config for a database [healthcheck.HealthChecker].
type HealthCheckerConfig struct {
	// Auth handles database access authentication.
	Auth common.Auth
	// AuthClient is the cluster auth server client.
	AuthClient *authclient.Client
	// Clock is an optional clock to use
	Clock clockwork.Clock
	// Database is the database to health check.
	Database types.Database
	// GCPClients are used to access GCP API for health checks.
	GCPClients GCPClients
	// Log is an optional logger.
	Log *slog.Logger
	// UpdateProxiedDatabase finds the proxied database by name and uses the
	// provided function to update the database's status. Returns
	// trace.NotFound if the name is not found otherwise forwards the error
	// from the provided callback function.
	UpdateProxiedDatabase func(string, func(types.Database) error) error
}

func (cfg *HealthCheckerConfig) checkAndSetDefaults() error {
	switch {
	case cfg.Auth == nil:
		return trace.BadParameter("missing Auth")
	case cfg.AuthClient == nil:
		return trace.BadParameter("missing AuthClient")
	case cfg.Database == nil:
		return trace.BadParameter("missing Database")
	case cfg.GCPClients == nil:
		return trace.BadParameter("missing GCPClients")
	case cfg.UpdateProxiedDatabase == nil:
		return trace.BadParameter("missing UpdateProxiedDatabase")
	}

	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	return nil
}

// GCPClients are clients used to resolve GCP endpoints.
type GCPClients interface {
	// GetGCPSQLAdminClient returns GCP Cloud SQL Admin client.
	GetGCPSQLAdminClient(context.Context) (gcp.SQLAdminClient, error)
	// GetGCPAlloyDBClient returns GCP AlloyDB Admin client.
	GetGCPAlloyDBClient(context.Context) (gcp.AlloyDBAdminClient, error)
}
