/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package common

import (
	"context"
	"log/slog"
	"sync"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/srv/db/common/enterprise"
)

var (
	// engines is a global database engines registry.
	engines map[string]EngineFn
	// enginesMu protects access to the global engines registry map.
	enginesMu sync.RWMutex
)

// EngineFn defines a database engine constructor function.
type EngineFn func(EngineConfig) Engine

// RegisterEngine registers a new engine constructor.
func RegisterEngine(fn EngineFn, names ...string) {
	enginesMu.Lock()
	defer enginesMu.Unlock()
	if engines == nil {
		engines = make(map[string]EngineFn)
	}
	for _, name := range names {
		engines[name] = fn
	}
}

// GetEngine returns a new engine for the provided configuration.
func GetEngine(db types.Database, conf EngineConfig) (Engine, error) {
	if err := conf.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	conf.Auth = newReportingAuth(db, conf.Auth)
	enginesMu.RLock()
	name := db.GetProtocol()
	engineFn := engines[name]
	enginesMu.RUnlock()
	if engineFn == nil {
		return nil, trace.NotFound("database engine %q is not registered", name)
	}
	engine, err := newReportingEngine(reporterConfig{
		engine:    engineFn(conf),
		component: teleport.ComponentDatabase,
		database:  db,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return engine, nil
}

// CheckEngines checks if provided engine names are registered.
func CheckEngines(names ...string) error {
	enginesMu.RLock()
	defer enginesMu.RUnlock()
	for _, name := range names {
		if err := enterprise.ProtocolValidation(name); err != nil {
			// Don't assert Enterprise protocol is a build is OSS
			continue
		}
		if engines[name] == nil {
			return trace.NotFound("database engine %q is not registered", name)
		}
	}
	return nil
}

// EngineConfig is the common configuration every database engine uses.
type EngineConfig struct {
	// Auth handles database access authentication.
	Auth Auth
	// Audit emits database access audit events.
	Audit Audit
	// AuthClient is the cluster auth server client.
	AuthClient *authclient.Client
	// AWSConfigProvider provides [aws.Config] for AWS SDK service clients.
	AWSConfigProvider awsconfig.Provider
	// CloudClients provides access to cloud API clients.
	CloudClients cloud.Clients
	// Context is the database server close context.
	Context context.Context
	// Clock is the clock interface.
	Clock clockwork.Clock
	// Log is used for logging.
	Log *slog.Logger
	// Users handles database users.
	Users Users
	// DataDir is the Teleport data directory
	DataDir string
	// GetUserProvisioner is automatic database users creation handler.
	GetUserProvisioner func(AutoUsers) *UserProvisioner
	// UpdateProxiedDatabase finds the proxied database by name and uses the
	// provided function to update the database's status. Returns
	// trace.NotFound if the name is not found otherwise forwards the error
	// from the provided callback function.
	UpdateProxiedDatabase func(string, func(types.Database) error) error
}

// CheckAndSetDefaults validates the config and sets default values.
func (c *EngineConfig) CheckAndSetDefaults() error {
	if c.Auth == nil {
		return trace.BadParameter("engine config Auth is missing")
	}
	if c.Audit == nil {
		return trace.BadParameter("engine config Audit is missing")
	}
	if c.AuthClient == nil {
		return trace.BadParameter("engine config AuthClient is missing")
	}
	if c.AWSConfigProvider == nil {
		return trace.BadParameter("missing AWSConfigProvider")
	}
	if c.CloudClients == nil {
		return trace.BadParameter("engine config CloudClients are missing")
	}
	if c.Context == nil {
		c.Context = context.Background()
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.Log == nil {
		c.Log = slog.Default()
	}
	return nil
}
