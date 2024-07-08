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
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

type Objects interface {
	StartImporter(ctx context.Context, database types.Database) error
	StopImporter(database types.Database) error
}

type Config struct {
	AuthClient   *authclient.Client
	Auth         common.Auth
	CloudClients cloud.Clients

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

func (c *Config) CheckAndSetDefaults() error {
	if c.AuthClient == nil {
		return trace.BadParameter("missing parameter AuthClient")
	}
	if c.Auth == nil {
		return trace.BadParameter("missing parameter Auth")
	}
	if c.CloudClients == nil {
		return trace.BadParameter("missing parameter CloudClients")
	}
	if c.Log == nil {
		c.Log = slog.Default().With(teleport.ComponentKey, "db:obj_importer")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	if c.ScanInterval == 0 {
		c.ScanInterval = time.Minute * 5
	}
	if c.ObjectTTL == 0 {
		c.ObjectTTL = time.Minute * 60
	}
	if c.RefreshThreshold == 0 {
		c.RefreshThreshold = time.Minute * 15
	}

	return nil
}

type objects struct {
	cfg Config

	importerMap    map[string]context.CancelFunc
	importersMutex sync.RWMutex
}

func NewObjects(cfg Config) (Objects, error) {
	err := cfg.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	result := &objects{
		cfg:         cfg,
		importerMap: make(map[string]context.CancelFunc),
	}
	return result, nil
}

var _ Objects = (*objects)(nil)

// StartImporter starts a new importer for a given database.
// An error will be returned only in case of interface misuse, e.g. attempt to start the importer for same database twice.
// If the database configuration (protocol/type/parameters) is not supported, no error will be returned.
func (o *objects) StartImporter(ctx context.Context, database types.Database) error {
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
func (o *objects) StopImporter(database types.Database) error {
	o.importersMutex.Lock()
	defer o.importersMutex.Unlock()

	stopImporterFunc, ok := o.importerMap[database.GetName()]
	if !ok {
		return trace.NotFound("no importer found for database %q", database.GetName())
	}
	if stopImporterFunc != nil {
		stopImporterFunc()
	}
	delete(o.importerMap, database.GetName())
	return nil
}
