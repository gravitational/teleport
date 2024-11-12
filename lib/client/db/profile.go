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

// Package db contains methods for working with database connection profiles
// that combine connection parameters for a particular database.
//
// For Postgres it's the connection service file:
//
//	https://www.postgresql.org/docs/current/libpq-pgservice.html
//
// For MySQL it's the option file:
//
//	https://dev.mysql.com/doc/refman/8.0/en/option-files.html
package db

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/db/mysql"
	"github.com/gravitational/teleport/lib/client/db/postgres"
	"github.com/gravitational/teleport/lib/client/db/profile"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"
)

// Add updates database connection profile file.
func Add(ctx context.Context, tc *client.TeleportClient, db tlsca.RouteToDatabase, clientProfile client.ProfileStatus) error {
	if !IsSupported(db) {
		return nil
	}
	profileFile, err := load(tc, db)
	if err != nil {
		return trace.Wrap(err)
	}

	rootClusterName, err := tc.RootClusterName(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = add(tc, db, clientProfile, profileFile, rootClusterName)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func add(tc *client.TeleportClient, db tlsca.RouteToDatabase, clientProfile client.ProfileStatus, profileFile profile.ConnectProfileFile, rootCluster string) (*profile.ConnectProfile, error) {
	var host string
	var port int
	switch db.Protocol {
	case defaults.ProtocolPostgres:
		host, port = tc.PostgresProxyHostPort()
	case defaults.ProtocolMySQL:
		host, port = tc.MySQLProxyHostPort()
	default:
		return nil, trace.BadParameter("unknown database protocol: %q", db)
	}
	connectProfile := New(tc, db, clientProfile, rootCluster, host, port)
	err := profileFile.Upsert(*connectProfile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return connectProfile, nil
}

// New makes a new database connection profile.
func New(tc *client.TeleportClient, db tlsca.RouteToDatabase, clientProfile client.ProfileStatus, rootCluster string, host string, port int) *profile.ConnectProfile {
	return &profile.ConnectProfile{
		Name:       profileName(tc.SiteName, db.ServiceName),
		Host:       host,
		Port:       port,
		User:       db.Username,
		Database:   db.Database,
		Insecure:   tc.InsecureSkipVerify,
		CACertPath: clientProfile.CACertPathForCluster(rootCluster),
		CertPath:   clientProfile.DatabaseCertPathForCluster(tc.SiteName, db.ServiceName),
		KeyPath:    clientProfile.DatabaseKeyPathForCluster(tc.SiteName, db.ServiceName),
	}
}

// Env returns environment variables for the specified database profile.
func Env(tc *client.TeleportClient, db tlsca.RouteToDatabase) (map[string]string, error) {
	profileFile, err := load(tc, db)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	env, err := profileFile.Env(profileName(tc.SiteName, db.ServiceName))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return env, nil
}

// Delete removes the specified database connection profile.
func Delete(tc *client.TeleportClient, db tlsca.RouteToDatabase) error {
	if !IsSupported(db) {
		return nil
	}
	profileFile, err := load(tc, db)
	if err != nil {
		return trace.Wrap(err)
	}
	err = profileFile.Delete(profileName(tc.SiteName, db.ServiceName))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// IsSupported checks if provided database is supported.
func IsSupported(db tlsca.RouteToDatabase) bool {
	// Out of supported databases, only Postgres and MySQL have a concept
	// of the connection options file.
	switch db.Protocol {
	case defaults.ProtocolPostgres, defaults.ProtocolMySQL:
		return true
	default:
		return false
	}
}

// load loads the appropriate database connection profile.
func load(tc *client.TeleportClient, db tlsca.RouteToDatabase) (profile.ConnectProfileFile, error) {
	switch db.Protocol {
	case defaults.ProtocolPostgres:
		if tc.OverridePostgresServiceFilePath != "" {
			return postgres.LoadFromPath(tc.OverridePostgresServiceFilePath)
		}
		return postgres.Load()
	case defaults.ProtocolMySQL:
		if tc.OverrideMySQLOptionFilePath != "" {
			return mysql.LoadFromPath(tc.OverrideMySQLOptionFilePath)
		}
		return mysql.Load()
	}
	return nil, trace.BadParameter("unsupported database protocol %q",
		db.Protocol)
}

// profileName constructs the Postgres connection service name from the
// Teleport cluster name and the database service name.
func profileName(cluster, name string) string {
	return fmt.Sprintf("%v-%v", cluster, name)
}
