/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package postgres

import (
	"context"
	"time"

	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/sqlbk"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	// Ensure pgx driver is registered.
	_ "github.com/jackc/pgx/v4/stdlib"
)

const (
	// BackendName is the name of this backend.
	BackendName = "postgres"
	// AlternativeName is another name of this backend.
	AlternativeName = "cockroachdb"
)

// GetName returns BackendName (postgres).
func GetName() string {
	return BackendName
}

// New returns a Backend that speaks the PostgreSQL protocol when communicating
// with the database. The connection pool is ready and the database has been
// migrated to the most recent version upon return without an error.
func New(ctx context.Context, params backend.Params) (*sqlbk.Backend, error) {
	var cfg *Config
	err := utils.ObjectToStruct(params, &cfg)
	if err != nil {
		return nil, trace.BadParameter("invalid configuration: %v", err)
	}
	err = cfg.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sqlbk.New(ctx, &pgDriver{cfg: cfg})
}

// Config defines a configuration for the postgres backend.
type Config struct {
	sqlbk.Config

	// ConnMaxIdleTime sets the maximum amount of time a connection may be idle.
	// https://pkg.go.dev/database/sql#DB.SetConnMaxIdleTime
	ConnMaxIdleTime time.Duration `json:"conn_max_idle_time,omitempty"`

	// ConnMaxLifetime sets the maximum amount of time a connection may be reused.
	// https://pkg.go.dev/database/sql#DB.SetConnMaxLifetime
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime,omitempty"`

	// MaxIdleConns sets the maximum number of connections in the idle connection pool.
	// https://pkg.go.dev/database/sql#DB.SetMaxIdleConns
	MaxIdleConns int `json:"max_idle_conns,omitempty"`

	// SetMaxOpenConns sets the maximum number of open connections to the database.
	// https://pkg.go.dev/database/sql#DB.SetMaxOpenConns
	MaxOpenConns int `json:"max_open_conns,omitempty"`

	// Add configurations specific to this backend.
	//
	// AfterConnect pgconn.AfterConnectFunc `json:"-"`
	// DialFunc     pgconn.DialFunc         `json:"-"`
	// RuntimeParams struct {
	//   SearchPath string `json:"search_path"`
	// } `json:"runtime_params"`
}

// CheckAndSetDefaults validates required fields and sets default
// values for fields that have not been set.
func (c *Config) CheckAndSetDefaults() error {
	if c.MaxOpenConns == 0 {
		c.MaxOpenConns = DefaultMaxOpenConns
	}
	if c.ConnMaxIdleTime == 0 {
		c.ConnMaxIdleTime = DefaultConnMaxIdleTime
	}
	if c.ConnMaxLifetime == 0 {
		c.ConnMaxLifetime = DefaultConnMaxLifetime
	}
	if c.MaxIdleConns == 0 {
		c.MaxIdleConns = DefaultMaxIdleConns
	}
	if c.Log == nil {
		c.Log = logrus.WithFields(logrus.Fields{trace.Component: BackendName})
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	err := c.Config.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}

	err = validateDatabaseName(c.Database)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// validateDatabaseName returns true when name contains only alphanumeric and/or
// underscore/dollar characters, the first character is not a digit, and the
// name's length is less than MaxDatabaseNameLength (63 bytes).
func validateDatabaseName(name string) error {
	if MaxDatabaseNameLength <= len(name) {
		return trace.BadParameter("invalid PostgreSQL database name, length exceeds %d bytes. See https://www.postgresql.org/docs/14/sql-syntax-lexical.html.", MaxDatabaseNameLength)
	}
	for i, r := range name {
		switch {
		case 'A' <= r && r <= 'Z', 'a' <= r && r <= 'z', r == '_':
		case i > 0 && (r == '$' || '0' <= r && r <= '9'):
		default:
			return trace.BadParameter("invalid PostgreSQL database name: %v. See https://www.postgresql.org/docs/14/sql-syntax-lexical.html.", name)
		}
	}
	return nil
}

const (
	// DefaultConnMaxIdleTime means connections are not closed due to a
	// connection's idle time.
	DefaultConnMaxIdleTime = 0

	// DefaultConnMaxLifetime means connections are not closed due to a
	// connection's age.
	DefaultConnMaxLifetime = 0

	// DefaultMaxIdleConns means 2 idle connections are retained in the pool (same
	// configuration as the standard library). If MaxIdleConns <= 0, no idle
	// connections are retained.
	DefaultMaxIdleConns = 2

	// DefaultMaxOpenConns means the maximum number of open database connections
	// is 50.
	DefaultMaxOpenConns = 50

	// MaxDatabaseNameLength is the maximum PostgreSQL identifier length.
	// https://www.postgresql.org/docs/14/sql-syntax-lexical.html#SQL-SYNTAX-IDENTIFIERS
	MaxDatabaseNameLength = 63
)
