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
package db

import (
	"fmt"
	"os"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/db/postgres"
	"github.com/gravitational/teleport/lib/client/db/profile"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/gravitational/trace"
)

// Add updates database connection profile file.
func Add(tc *client.TeleportClient, db tlsca.RouteToDatabase, clientProfile client.ProfileStatus, quiet bool) error {
	profileFile, err := load(db)
	if err != nil {
		return trace.Wrap(err)
	}
	host, port := tc.WebProxyHostPort()
	connectProfile := profile.ConnectProfile{
		Name:       profileName(tc.SiteName, db.ServiceName),
		Host:       host,
		Port:       port,
		User:       db.Username,
		Database:   db.Database,
		Insecure:   tc.InsecureSkipVerify,
		CACertPath: clientProfile.CACertPath(),
		CertPath:   clientProfile.DatabaseCertPath(db.ServiceName),
		KeyPath:    clientProfile.KeyPath(),
	}
	err = profileFile.Upsert(connectProfile)
	if err != nil {
		return trace.Wrap(err)
	}
	if quiet {
		return nil
	}
	switch db.Protocol {
	case defaults.ProtocolPostgres:
		return postgres.Message.Execute(os.Stdout, connectProfile)
	}
	return nil
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
	}
	return nil, trace.BadParameter("unsupported database protocol %q",
		db.Protocol)
}

// profileName constructs the Postgres connection service name from the
// Teleport cluster name and the database service name.
func profileName(cluster, name string) string {
	return fmt.Sprintf("%v-%v", cluster, name)
}
