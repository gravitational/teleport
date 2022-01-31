/*
Copyright 2021 Gravitational, Inc.

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

// Package db contains methods for working with database connection profiles
// that combine connection parameters for a particular database.
//
// For Postgres it's the connection service file:
//   https://www.postgresql.org/docs/current/libpq-pgservice.html
//
// For MySQL it's the option file:
//   https://dev.mysql.com/doc/refman/8.0/en/option-files.html
package db

import (
	"fmt"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/db/mysql"
	"github.com/gravitational/teleport/lib/client/db/postgres"
	"github.com/gravitational/teleport/lib/client/db/profile"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/gravitational/trace"
)

// Add updates database connection profile file.
func Add(tc *client.TeleportClient, db tlsca.RouteToDatabase, clientProfile client.ProfileStatus) error {
	// Out of supported databases, only Postgres and MySQL have a concept
	// of the connection options file.
	switch db.Protocol {
	case defaults.ProtocolPostgres, defaults.ProtocolMySQL:
	default:
		return nil
	}
	profileFile, err := load(db)
	if err != nil {
		return trace.Wrap(err)
	}

	rootClusterName, err := tc.RootClusterName()
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
		KeyPath:    clientProfile.KeyPath(),
	}
}

// Env returns environment variables for the specified database profile.
func Env(tc *client.TeleportClient, db tlsca.RouteToDatabase) (map[string]string, error) {
	profileFile, err := load(db)
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
	// Out of supported databases, only Postgres and MySQL have a concept
	// of the connection options file.
	switch db.Protocol {
	case defaults.ProtocolPostgres, defaults.ProtocolMySQL:
	default:
		return nil
	}
	profileFile, err := load(db)
	if err != nil {
		return trace.Wrap(err)
	}
	err = profileFile.Delete(profileName(tc.SiteName, db.ServiceName))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// load loads the appropriate database connection profile.
func load(db tlsca.RouteToDatabase) (profile.ConnectProfileFile, error) {
	switch db.Protocol {
	case defaults.ProtocolPostgres:
		return postgres.Load()
	case defaults.ProtocolMySQL:
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
