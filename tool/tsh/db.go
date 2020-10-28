/*
Copyright 2020 Gravitational, Inc.

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

package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gravitational/teleport/lib/auth/proto"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/pgservicefile"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// onListDatabases handles "tsh db ls" command.
func onListDatabases(cf *CLIConf) {
	tc, err := makeClient(cf, false)
	if err != nil {
		utils.FatalError(err)
	}
	var servers []services.DatabaseServer
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		servers, err = tc.ListDatabaseServers(cf.Context)
		return trace.Wrap(err)
	})
	if err != nil {
		utils.FatalError(err)
	}
	sort.Slice(servers, func(i, j int) bool {
		return servers[i].GetName() < servers[j].GetName()
	})
	// Retrieve profile to be able to show which databases user is logged into.
	profile, err := client.StatusCurrent("", cf.Proxy)
	if err != nil {
		utils.FatalError(err)
	}
	showDatabases(servers, profile.Databases, cf.Verbose)
}

// onDatabaseLogin handles "tsh db login" command.
func onDatabaseLogin(cf *CLIConf) {
	tc, err := makeClient(cf, false)
	if err != nil {
		utils.FatalError(err)
	}
	var servers []services.DatabaseServer
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		servers, err = tc.ListDatabaseServersFor(cf.Context, cf.DatabaseService)
		return trace.Wrap(err)
	})
	if err != nil {
		utils.FatalError(err)
	}
	if len(servers) == 0 {
		utils.FatalError(trace.NotFound(
			"database %q not found, use 'tsh db ls' to see registered databases", cf.DatabaseService))
	}
	err = databaseLogin(cf, tc, tlsca.RouteToDatabase{
		ServiceName: cf.DatabaseService,
		Protocol:    servers[0].GetProtocol(),
		Username:    cf.DatabaseUser,
		Database:    cf.DatabaseName,
	}, false)
	if err != nil {
		utils.FatalError(err)
	}
}

func databaseLogin(cf *CLIConf, tc *client.TeleportClient, db tlsca.RouteToDatabase, quiet bool) error {
	log.Debugf("Fetching database access certificate for %s on cluster %v.", db, tc.SiteName)
	profile, err := client.StatusCurrent("", cf.Proxy)
	if err != nil {
		return trace.Wrap(err)
	}
	err = tc.ReissueUserCerts(cf.Context, client.ReissueParams{
		RouteToCluster: tc.SiteName,
		RouteToDatabase: proto.RouteToDatabase{
			ServiceName: db.ServiceName,
			Protocol:    db.Protocol,
			Username:    db.Username,
			Database:    db.Database,
		},
		AccessRequests: profile.ActiveRequests.AccessRequests,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	// Refresh the profile.
	profile, err = client.StatusCurrent("", cf.Proxy)
	if err != nil {
		utils.FatalError(err)
	}
	// Perform database-specific actions such as updating Postgres
	// connection service file.
	switch db.Protocol {
	case defaults.ProtocolPostgres:
		err := pgservicefile.Add(tc.SiteName, db.ServiceName, db.Username, db.Database, *profile, quiet)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// fetchDatabaseCreds is called as a part of tsh login to refresh database
// access certificates for databases the current profile is logged into.
func fetchDatabaseCreds(cf *CLIConf, tc *client.TeleportClient) error {
	profile, err := client.StatusCurrent("", cf.Proxy)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, db := range profile.Databases {
		if err := databaseLogin(cf, tc, db, true); err != nil {
			log.WithError(err).Errorf("Failed to fetch database access certificate for %s.", db)
			if err := databaseLogout(tc, db); err != nil {
				log.WithError(err).Errorf("Failed to log out of database %s.", db)
			}
		}
	}
	return nil
}

// onDatabaseLogout handles "tsh db logout" command.
func onDatabaseLogout(cf *CLIConf) {
	tc, err := makeClient(cf, false)
	if err != nil {
		utils.FatalError(err)
	}
	profile, err := client.StatusCurrent("", cf.Proxy)
	if err != nil {
		utils.FatalError(err)
	}
	var logout []tlsca.RouteToDatabase
	// If database name wasn't given on the command line, log out of all.
	if cf.DatabaseService == "" {
		logout = profile.Databases
	} else {
		for _, db := range profile.Databases {
			if db.ServiceName == cf.DatabaseService {
				logout = append(logout, db)
			}
		}
		if len(logout) == 0 {
			utils.FatalError(trace.BadParameter("Not logged into database %q",
				tc.DatabaseService))
		}
	}
	for _, db := range logout {
		if err := databaseLogout(tc, db); err != nil {
			utils.FatalError(err)
		}
	}
	if len(logout) == 1 {
		fmt.Println("Logged out of database", logout[0].ServiceName)
	} else {
		fmt.Println("Logged out of all databases")
	}
}

func databaseLogout(tc *client.TeleportClient, db tlsca.RouteToDatabase) error {
	// First perform database-specific actions, such as remove connection
	// information from Postgres service file.
	switch db.Protocol {
	case defaults.ProtocolPostgres:
		err := pgservicefile.Delete(tc.SiteName, db.ServiceName)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	// Then remove the certificate from the keystore.
	err := tc.LogoutDatabase(db.ServiceName)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// onDatabaseEnv handles "tsh db env" command.
func onDatabaseEnv(cf *CLIConf) {
	profile, err := client.StatusCurrent("", cf.Proxy)
	if err != nil {
		utils.FatalError(err)
	}
	if len(profile.Databases) == 0 {
		utils.FatalError(trace.BadParameter("Please login using 'tsh db login' first"))
	}
	database := cf.DatabaseService
	if database == "" {
		services := profile.DatabaseServices()
		if len(services) > 1 {
			utils.FatalError(trace.BadParameter("Multiple databases are available (%v), please select the one to print environment for via --db flag",
				strings.Join(services, ", ")))
		}
		database = services[0]
	}
	env, err := pgservicefile.Env(profile.Cluster, database)
	if err != nil {
		utils.FatalError(err)
	}
	for k, v := range env {
		fmt.Printf("export %v=%v\n", k, v)
	}
}
