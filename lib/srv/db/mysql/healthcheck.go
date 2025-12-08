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

package mysql

import (
	"context"
	"net"
	"os"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/healthcheck"
	"github.com/gravitational/teleport/lib/srv/db/healthchecks"
	"github.com/gravitational/teleport/lib/srv/db/mysql/protocol"
)

const (
	// defaultHealthCheckUser is the user name that the health checker will
	// attempt to authenticate as. It may be overridden with environment variable.
	defaultHealthCheckUser = "teleport-healthchecker"
	// healthCheckUserEnvVar is used to override the default health check user.
	healthCheckUserEnvVar = "TELEPORT_UNSTABLE_MYSQL_DB_HEALTH_CHECK_DEFAULT_USER"
)

// getHealthCheckDBUser returns the user name to use for health checks.
// The name can be sourced from environment variable, database config, or
// the default health check user.
func getHealthCheckDBUser(database types.Database) string {
	user := defaultHealthCheckUser
	if val := os.Getenv(healthCheckUserEnvVar); val != "" {
		user = val
	}
	if admin := database.GetAdminUser(); admin.Name != "" {
		user = admin.Name
	}
	if database.IsCloudSQL() {
		return databaseUserToGCPServiceAccount(database, user)
	}
	return user
}

// NewHealthChecker creates a new [HealthChecker].
func NewHealthChecker(ctx context.Context, cfg healthchecks.HealthCheckerConfig) (healthcheck.HealthChecker, error) {
	return newHealthChecker(cfg)
}

func newHealthChecker(cfg healthchecks.HealthCheckerConfig) (*HealthChecker, error) {
	databaseUser := getHealthCheckDBUser(cfg.Database)
	cfg.Log = cfg.Log.With(
		"db", cfg.Database.GetName(),
		"db_user", databaseUser,
	)
	connector, err := newConnector(connectorConfig{
		auth:         cfg.Auth,
		authClient:   cfg.AuthClient,
		clock:        cfg.Clock,
		gcpClients:   cfg.GCPClients,
		log:          cfg.Log,
		database:     cfg.Database,
		databaseName: "", // no default database is necessary
		databaseUser: databaseUser,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &HealthChecker{
		cfg:       cfg,
		connector: connector,
	}, nil
}

// HealthChecker provides health checks for a MySQL database. To avoid MySQL
// host blocking due to aborted connections this health checker attempts to
// authenticate as a user, but authentication failure is normal and expected
// behavior when the health check user does not exist. Any failure after
// successfully dialing the database endpoint is ignored, because database
// health checks only check TCP connectivity.
type HealthChecker struct {
	cfg       healthchecks.HealthCheckerConfig
	connector *connector
}

// CheckHealth checks the health of the target database.
func (h *HealthChecker) CheckHealth(ctx context.Context) ([]string, error) {
	if err := h.connect(ctx); err != nil {
		if isDialError(err) {
			// only return dial errors, because database health checks only
			// check connectivity to the database endpoint.
			return []string{h.getTargetAddress(ctx)}, trace.Wrap(err)
		}
		h.cfg.Log.DebugContext(ctx, "Failed to connect as health checker", "error", err)
	}
	return []string{h.getTargetAddress(ctx)}, nil
}

func (h *HealthChecker) connect(ctx context.Context) error {
	// TODO(gavin): implement Teleport and GCP ephemeral client cert caching
	certExpiry := h.cfg.Clock.Now().Add(time.Hour)
	conn, err := h.connector.connect(ctx, certExpiry, h.readServerVersion)
	if err != nil {
		return trace.Wrap(err)
	}
	_ = conn.Quit()
	return nil
}

func (h *HealthChecker) readServerVersion(ctx context.Context, conn net.Conn) {
	version, err := protocol.ReadMySQLVersion(ctx, conn)
	if err != nil {
		h.cfg.Log.WarnContext(ctx, "Failed to fetch the MySQL version",
			"db", h.cfg.Database.GetName(),
			"error", err,
		)
		return
	}
	if err := updateServerVersion(ctx,
		h.cfg.Log,
		h.cfg.Database,
		version,
		h.cfg.UpdateProxiedDatabase,
	); err != nil {
		if trace.IsNotFound(err) {
			// not found error can occur if we fetch the version before the
			// database has finished registering - we will retry later, ignore it
			return
		}
		h.cfg.Log.WarnContext(ctx, "Failed to update the MySQL server version",
			"error", err,
		)
	}
}

// GetProtocol returns the network protocol used for checking health.
// This health checker only reports TCP dialing errors.
func (h *HealthChecker) GetProtocol() types.TargetHealthProtocol {
	return types.TargetHealthProtocolTCP
}

func (h *HealthChecker) getTargetAddress(ctx context.Context) string {
	if h.cfg.Database.IsCloudSQL() {
		requireSSL, err := h.connector.gcpAuth.checkSSLRequired(ctx)
		if err != nil {
			h.cfg.Log.DebugContext(ctx, "Failed to check database SSL requirement",
				"error", err,
			)
		} else if requireSSL {
			return getGCPTLSAddress(h.cfg.Database.GetURI())
		}
	}
	return h.cfg.Database.GetURI()
}
