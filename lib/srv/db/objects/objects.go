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

package objects

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/databaseobject"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

type Objects interface {
	StartImporter(ctx context.Context, database types.Database) error
	StopImporter(databaseName string) error
}

type Config struct {
	DatabaseObjectClient *databaseobject.Client
	ImportRules          ImportRulesReader
	Auth                 common.Auth
	GCPClients           cloud.GCPClients

	// ScanInterval specifies how often the database is scanned.
	// A higher ScanInterval reduces the load on the database and database agent,
	// but increases the delay in detecting schema changes or updates from new import rules.
	ScanInterval time.Duration

	// ObjectTTL defines TTL for a newly created object.
	// A higher ObjectTTL reduces the backend load and should be significantly larger than ScanInterval.
	// Setting a TTL for database objects ensures cleanup if the database becomes unavailable or the database agent stops performing scans.
	ObjectTTL time.Duration

	// RefreshThreshold sets the minimum remaining TTL for an object to qualify for a TTL refresh.
	RefreshThreshold time.Duration

	Clock clockwork.Clock
	Log   *slog.Logger
}

// loadEnvVar parses the named env vars as a duration.
func (c *Config) loadEnvVar(ctx context.Context, name string) (bool, time.Duration) {
	envVar := os.Getenv(name)
	if envVar == "" {
		return false, 0
	}
	if envVar == "never" {
		return true, 0
	}

	interval, err := time.ParseDuration(envVar)
	if err != nil {
		c.Log.ErrorContext(ctx, "Failed to parse env var, override not applied.", "name", name, "value", envVar)
	}

	return true, interval
}

func (c *Config) loadEnvVarOverrides(ctx context.Context) {
	needInfo := false
	// overriding scan interval modifies the other variables, but not the other way around.
	if found, value := c.loadEnvVar(ctx, "TELEPORT_UNSTABLE_DB_OBJECTS_SCAN_INTERVAL"); found {
		// multipliers 12 and 3 mimic the interval length proportions of default configuration (15/180/45 minutes).
		c.ScanInterval = value
		c.ObjectTTL = value * 12
		c.RefreshThreshold = value * 3
		needInfo = true
	}
	if found, value := c.loadEnvVar(ctx, "TELEPORT_UNSTABLE_DB_OBJECTS_OBJECT_TTL"); found {
		c.ObjectTTL = value
		needInfo = true
	}
	if found, value := c.loadEnvVar(ctx, "TELEPORT_UNSTABLE_DB_OBJECTS_REFRESH_THRESHOLD"); found {
		c.RefreshThreshold = value
		needInfo = true
	}

	if needInfo {
		c.Log.InfoContext(ctx, "Applied env var overrides",
			"scan_interval", c.ScanInterval.String(),
			"object_ttl", c.ObjectTTL.String(),
			"refresh_threshold", c.RefreshThreshold.String(),
		)
	}
}

func (c *Config) CheckAndSetDefaults(ctx context.Context) error {
	if c.DatabaseObjectClient == nil {
		return trace.BadParameter("missing parameter DatabaseObjectClient")
	}
	if c.ImportRules == nil {
		return trace.BadParameter("missing parameter ImportRules")
	}
	if c.Auth == nil {
		return trace.BadParameter("missing parameter Auth")
	}
	if c.GCPClients == nil {
		return trace.BadParameter("missing parameter GCPClients")
	}
	if c.Log == nil {
		c.Log = slog.Default().With(teleport.ComponentKey, "db:obj_importer")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.ScanInterval == 0 {
		c.ScanInterval = time.Minute * 15
	}
	if c.ObjectTTL == 0 {
		c.ObjectTTL = time.Minute * 180
	}
	if c.RefreshThreshold == 0 {
		c.RefreshThreshold = time.Minute * 45
	}

	c.loadEnvVarOverrides(ctx)

	return nil
}

func (c *Config) disabled() bool {
	return c.ObjectTTL <= 0 || c.ScanInterval <= 0
}

type objects struct {
	cfg Config

	importerMap    map[string]context.CancelFunc
	importersMutex sync.RWMutex
}

func NewObjects(ctx context.Context, cfg Config) (Objects, error) {
	err := cfg.CheckAndSetDefaults(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	result := &objects{
		cfg:         cfg,
		importerMap: make(map[string]context.CancelFunc),
	}
	if result.disabled() {
		cfg.Log.WarnContext(ctx, "Objects importer is disabled through config.")
	}
	return result, nil
}

var _ Objects = (*objects)(nil)

// StartImporter starts a new importer for a given database.
// An error will be returned only in case of interface misuse, e.g. attempt to start the importer for same database twice.
// If the database configuration (protocol/type/parameters) is not supported, no error will be returned.
func (o *objects) StartImporter(ctx context.Context, database types.Database) error {
	if o.disabled() {
		return nil
	}

	o.importersMutex.Lock()
	defer o.importersMutex.Unlock()

	if _, ok := o.importerMap[database.GetName()]; ok {
		return trace.AlreadyExists("importer for database %q already started", database.GetName())
	}
	stopImporterFunc, err := startDatabaseImporter(ctx, o.cfg, database)
	if err != nil {
		// register dummy "stop" function to avoid errors on shutdown.
		o.importerMap[database.GetName()] = func() {}
		return trace.Wrap(err)
	}
	o.importerMap[database.GetName()] = stopImporterFunc
	return nil
}

// StopImporter stops the running importer for a given database.
func (o *objects) StopImporter(name string) error {
	if o.disabled() {
		return nil
	}

	o.importersMutex.Lock()
	defer o.importersMutex.Unlock()

	stopImporterFunc, ok := o.importerMap[name]
	if !ok {
		return trace.NotFound("no importer found for database %q", name)
	}
	if stopImporterFunc != nil {
		stopImporterFunc()
	}
	delete(o.importerMap, name)
	return nil
}

func (o *objects) disabled() bool {
	return o.cfg.disabled()
}
