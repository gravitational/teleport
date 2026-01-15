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

package postgres

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strconv"

	"github.com/gravitational/trace"
	"github.com/jackc/pgconn"

	"github.com/gravitational/teleport/api/types"
	gcputils "github.com/gravitational/teleport/api/utils/gcp"
	"github.com/gravitational/teleport/lib/healthcheck"
	"github.com/gravitational/teleport/lib/srv/db/healthchecks"
)

// NewHealthChecker returns an endpoint health checker.
func NewHealthChecker(_ context.Context, config healthchecks.HealthCheckerConfig) (healthcheck.HealthChecker, error) {
	// special handling for AlloyDB
	if config.Database.GetType() == types.DatabaseTypeAlloyDB {
		return newAlloyDBEndpointsResolver(config.Database, config.GCPClients)
	}

	resolver, err := newEndpointsResolver(config.Database.GetURI())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return healthcheck.NewTargetDialer(resolver.Resolve), nil
}

func newAlloyDBEndpointsResolver(db types.Database, clients healthchecks.GCPClients) (healthcheck.HealthChecker, error) {
	serverPort := strconv.Itoa(alloyDBServerProxyPort)

	info, err := gcputils.ParseAlloyDBConnectionURI(db.GetURI())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if db.GetGCP().AlloyDB.EndpointOverride != "" {
		addr := net.JoinHostPort(db.GetGCP().AlloyDB.EndpointOverride, serverPort)
		return healthcheck.NewTargetDialer(func(context.Context) ([]string, error) {
			return []string{addr}, nil
		}), nil
	}

	return healthcheck.NewTargetDialer(func(ctx context.Context) ([]string, error) {
		adminClient, err := clients.GetAlloyDBClient(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		addr, err := adminClient.GetEndpointAddress(ctx, *info, db.GetGCP().AlloyDB.EndpointType)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return []string{net.JoinHostPort(addr, serverPort)}, nil
	}), nil
}

func newEndpointsResolver(uri string) (healthcheck.EndpointsResolverFunc, error) {
	config, err := pgconn.ParseConfig(fmt.Sprintf("postgres://%s", uri))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	addrs := make([]string, 0, len(config.Fallbacks)+1)
	hostPort := net.JoinHostPort(config.Host, strconv.Itoa(int(config.Port)))
	addrs = append(addrs, hostPort)
	for _, fb := range config.Fallbacks {
		hostPort := net.JoinHostPort(fb.Host, strconv.Itoa(int(fb.Port)))
		// pgconn duplicates the host/port in its fallbacks for some reason, so
		// we de-duplicate and preserve the fallback order
		if !slices.Contains(addrs, hostPort) {
			addrs = append(addrs, hostPort)
		}
	}
	return func(context.Context) ([]string, error) {
		return addrs, nil
	}, nil
}
