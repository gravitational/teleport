/*
Copyright 2020-2021 Gravitational, Inc.

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
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	dbprofile "github.com/gravitational/teleport/lib/client/db"
	"github.com/gravitational/teleport/lib/client/db/dbcmd"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
)

// onListDatabases implements "tsh db ls" command.
func onListDatabases(cf *CLIConf) error {
	if cf.ListAll {
		return trace.Wrap(listDatabasesAllClusters(cf))
	}
	tc, err := makeClient(cf, false)
	if err != nil {
		return trace.Wrap(err)
	}
	var databases []types.Database
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		databases, err = tc.ListDatabases(cf.Context, nil /* custom filter */)
		return trace.Wrap(err)
	})
	if err != nil {
		if utils.IsPredicateError(err) {
			return trace.Wrap(utils.PredicateError{Err: err})
		}
		return trace.Wrap(err)
	}

	proxy, err := tc.ConnectToProxy(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}

	cluster, err := proxy.ConnectToCurrentCluster(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}
	defer cluster.Close()

	// Retrieve profile to be able to show which databases user is logged into.
	profile, err := client.StatusCurrent(cf.HomePath, cf.Proxy, cf.IdentityFileIn)
	if err != nil {
		return trace.Wrap(err)
	}

	// get roles and traits. default to the set from profile, try to get up-to-date version from server point of view.
	roles := profile.Roles
	traits := profile.Traits

	// GetCurrentUser() may not be implemented, fail gracefully.
	user, err := cluster.GetCurrentUser(cf.Context)
	if err == nil {
		roles = user.GetRoles()
		traits = user.GetTraits()
	} else {
		log.Debugf("Failed to fetch current user information: %v.", err)
	}

	// get the role definition for all roles of user.
	// this may only fail if the role which we are looking for does not exist, or we don't have access to it.
	// example scenario when this may happen:
	// 1. we have set of roles [foo bar] from profile.
	// 2. the cluster is remote and maps the [foo, bar] roles to single role [guest]
	// 3. the remote cluster doesn't implement GetCurrentUser(), so we have no way to learn of [guest].
	// 4. services.FetchRoles([foo bar], ..., ...) fails as [foo bar] does not exist on remote cluster.
	roleSet, err := services.FetchRoles(roles, cluster, traits)
	if err != nil {
		log.Debugf("Failed to fetch user roles: %v.", err)
	}

	sort.Slice(databases, func(i, j int) bool {
		return databases[i].GetName() < databases[j].GetName()
	})

	activeDatabases, err := profile.DatabasesForCluster(tc.SiteName)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(showDatabases(cf.SiteName, databases, activeDatabases, roleSet, cf.Format, cf.Verbose))
}

type databaseListing struct {
	Proxy    string           `json:"proxy"`
	Cluster  string           `json:"cluster"`
	roleSet  services.RoleSet `json:"-"`
	Database types.Database   `json:"database"`
}

type databaseListings []databaseListing

func (l databaseListings) Len() int {
	return len(l)
}

func (l databaseListings) Less(i, j int) bool {
	if l[i].Proxy != l[j].Proxy {
		return l[i].Proxy < l[j].Proxy
	}
	if l[i].Cluster != l[j].Cluster {
		return l[i].Cluster < l[j].Cluster
	}
	return l[i].Database.GetName() < l[j].Database.GetName()
}

func (l databaseListings) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func listDatabasesAllClusters(cf *CLIConf) error {
	var dbListings databaseListings
	err := forEachProfile(cf, func(tc *client.TeleportClient, profile *client.ProfileStatus) error {
		result, err := tc.ListDatabasesAllClusters(cf.Context, nil /* custom filter */)
		if err != nil {
			return trace.Wrap(err)
		}

		proxy, err := tc.ConnectToProxy(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		var errors []error
		for clusterName, databases := range result {
			cluster, err := proxy.ConnectToCluster(cf.Context, clusterName)
			if err != nil {
				errors = append(errors, err)
				continue
			}
			roleSet, err := services.FetchRoles(profile.Roles, cluster, profile.Traits)
			if err != nil {
				errors = append(errors, err)
				continue
			}
			for _, database := range databases {
				dbListings = append(dbListings, databaseListing{
					Proxy:    profile.ProxyURL.Host,
					Cluster:  clusterName,
					roleSet:  roleSet,
					Database: database,
				})
			}
		}
		return trace.NewAggregate(errors...)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	sort.Sort(dbListings)

	profile, err := client.StatusCurrent(cf.HomePath, cf.Proxy, cf.IdentityFileIn)
	if err != nil {
		return trace.Wrap(err)
	}
	var active []tlsca.RouteToDatabase
	if profile != nil {
		active = profile.Databases
	}

	format := strings.ToLower(cf.Format)
	switch format {
	case teleport.Text, "":
		printDatabasesWithClusters(cf.SiteName, dbListings, active, cf.Verbose)
	case teleport.JSON, teleport.YAML:
		out, err := serializeDatabasesAllClusters(dbListings, format)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println(out)
	default:
		return trace.BadParameter("unsupported format %q", format)
	}
	return nil
}

// onDatabaseLogin implements "tsh db login" command.
func onDatabaseLogin(cf *CLIConf) error {
	tc, err := makeClient(cf, false)
	if err != nil {
		return trace.Wrap(err)
	}
	database, err := getDatabase(cf, tc, cf.DatabaseService)
	if err != nil {
		return trace.Wrap(err)
	}
	err = databaseLogin(cf, tc, tlsca.RouteToDatabase{
		ServiceName: cf.DatabaseService,
		Protocol:    database.GetProtocol(),
		Username:    cf.DatabaseUser,
		Database:    cf.DatabaseName,
	}, false)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func databaseLogin(cf *CLIConf, tc *client.TeleportClient, db tlsca.RouteToDatabase, quiet bool) error {
	log.Debugf("Fetching database access certificate for %s on cluster %v.", db, tc.SiteName)
	// When generating certificate for MongoDB access, database username must
	// be encoded into it. This is required to be able to tell which database
	// user to authenticate the connection as.
	if db.Protocol == defaults.ProtocolMongoDB && db.Username == "" {
		return trace.BadParameter("please provide the database user name using --db-user flag")
	}
	if db.Protocol == defaults.ProtocolRedis && db.Username == "" {
		// Default to "default" in the same way as Redis does. We need the username to check access on our side.
		// ref: https://redis.io/commands/auth
		db.Username = defaults.DefaultRedisUsername
	}

	profile, err := client.StatusCurrent(cf.HomePath, cf.Proxy, cf.IdentityFileIn)
	if err != nil {
		return trace.Wrap(err)
	}

	// Identity files themselves act as the database credentials (if any), so
	// don't bother fetching new certs.
	if profile.IsVirtual {
		log.Info("Note: already logged in due to an identity file (`-i ...`); will only update database config files.")
	} else {
		var key *client.Key
		if err = client.RetryWithRelogin(cf.Context, tc, func() error {
			key, err = tc.IssueUserCertsWithMFA(cf.Context, client.ReissueParams{
				RouteToCluster: tc.SiteName,
				RouteToDatabase: proto.RouteToDatabase{
					ServiceName: db.ServiceName,
					Protocol:    db.Protocol,
					Username:    db.Username,
					Database:    db.Database,
				},
				AccessRequests: profile.ActiveRequests.AccessRequests,
			})
			return trace.Wrap(err)
		}); err != nil {
			return trace.Wrap(err)
		}
		if err = tc.LocalAgent().AddDatabaseKey(key); err != nil {
			return trace.Wrap(err)
		}
	}

	// Refresh the profile.
	profile, err = client.StatusCurrent(cf.HomePath, cf.Proxy, cf.IdentityFileIn)
	if err != nil {
		return trace.Wrap(err)
	}
	// Update the database-specific connection profile file.
	err = dbprofile.Add(cf.Context, tc, db, *profile)
	if err != nil {
		return trace.Wrap(err)
	}
	// Print after-connect message.
	if !quiet {
		fmt.Println(formatDatabaseConnectMessage(cf.SiteName, db))
		return nil
	}
	return nil
}

// onDatabaseLogout implements "tsh db logout" command.
func onDatabaseLogout(cf *CLIConf) error {
	tc, err := makeClient(cf, false)
	if err != nil {
		return trace.Wrap(err)
	}
	profile, err := client.StatusCurrent(cf.HomePath, cf.Proxy, cf.IdentityFileIn)
	if err != nil {
		return trace.Wrap(err)
	}
	activeDatabases, err := profile.DatabasesForCluster(tc.SiteName)
	if err != nil {
		return trace.Wrap(err)
	}

	if profile.IsVirtual {
		log.Info("Note: an identity file is in use (`-i ...`); will only update database config files.")
	}

	var logout []tlsca.RouteToDatabase
	// If database name wasn't given on the command line, log out of all.
	if cf.DatabaseService == "" {
		logout = activeDatabases
	} else {
		for _, db := range activeDatabases {
			if db.ServiceName == cf.DatabaseService {
				logout = append(logout, db)
			}
		}
		if len(logout) == 0 {
			return trace.BadParameter("Not logged into database %q",
				tc.DatabaseService)
		}
	}
	for _, db := range logout {
		if err := databaseLogout(tc, db, profile.IsVirtual); err != nil {
			return trace.Wrap(err)
		}
	}
	if len(logout) == 1 {
		fmt.Println("Logged out of database", logout[0].ServiceName)
	} else {
		fmt.Println("Logged out of all databases")
	}
	return nil
}

func databaseLogout(tc *client.TeleportClient, db tlsca.RouteToDatabase, virtual bool) error {
	// First remove respective connection profile.
	err := dbprofile.Delete(tc, db)
	if err != nil {
		return trace.Wrap(err)
	}

	// Then remove the certificate from the keystore, but only for real
	// profiles.
	if !virtual {
		err = tc.LogoutDatabase(db.ServiceName)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// onDatabaseEnv implements "tsh db env" command.
func onDatabaseEnv(cf *CLIConf) error {
	tc, err := makeClient(cf, false)
	if err != nil {
		return trace.Wrap(err)
	}
	database, err := pickActiveDatabase(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	env, err := dbprofile.Env(tc, *database)
	if err != nil {
		return trace.Wrap(err)
	}

	format := strings.ToLower(cf.Format)
	switch format {
	case dbFormatText, "":
		for k, v := range env {
			fmt.Printf("export %v=%v\n", k, v)
		}
	case dbFormatJSON, dbFormatYAML:
		out, err := serializeDatabaseEnvironment(env, format)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println(out)
	default:
		return trace.BadParameter("unsupported format %q", cf.Format)
	}

	return nil
}

func serializeDatabaseEnvironment(env map[string]string, format string) (string, error) {
	var out []byte
	var err error
	if format == dbFormatJSON {
		out, err = utils.FastMarshalIndent(env, "", "  ")
	} else {
		out, err = yaml.Marshal(env)
	}
	return string(out), trace.Wrap(err)
}

// onDatabaseConfig implements "tsh db config" command.
func onDatabaseConfig(cf *CLIConf) error {
	tc, err := makeClient(cf, false)
	if err != nil {
		return trace.Wrap(err)
	}
	profile, err := client.StatusCurrent(cf.HomePath, cf.Proxy, cf.IdentityFileIn)
	if err != nil {
		return trace.Wrap(err)
	}
	database, err := pickActiveDatabase(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	rootCluster, err := tc.RootClusterName(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}
	// Postgres proxy listens on web proxy port while MySQL proxy listens on
	// a separate port due to the specifics of the protocol.
	var host string
	var port int
	switch database.Protocol {
	case defaults.ProtocolPostgres, defaults.ProtocolCockroachDB:
		host, port = tc.PostgresProxyHostPort()
	case defaults.ProtocolMySQL:
		host, port = tc.MySQLProxyHostPort()
	case defaults.ProtocolMongoDB, defaults.ProtocolRedis, defaults.ProtocolSQLServer, defaults.ProtocolSnowflake:
		host, port = tc.WebProxyHostPort()
	default:
		return trace.BadParameter("unknown database protocol: %q", database)
	}

	format := strings.ToLower(cf.Format)
	switch format {
	case dbFormatCommand:
		cmd, err := dbcmd.NewCmdBuilder(tc, profile, database, rootCluster,
			dbcmd.WithPrintFormat(),
			dbcmd.WithLogger(log),
		).GetConnectCommand()
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println(strings.Join(cmd.Env, " "), cmd.Path, strings.Join(cmd.Args[1:], " "))
	case dbFormatJSON, dbFormatYAML:
		configInfo := &dbConfigInfo{
			database.ServiceName, host, port, database.Username,
			database.Database, profile.CACertPathForCluster(rootCluster),
			profile.DatabaseCertPathForCluster(tc.SiteName, database.ServiceName),
			profile.KeyPath(),
		}
		out, err := serializeDatabaseConfig(configInfo, format)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println(out)
	default:
		fmt.Printf(`Name:      %v
Host:      %v
Port:      %v
User:      %v
Database:  %v
CA:        %v
Cert:      %v
Key:       %v
`,
			database.ServiceName, host, port, database.Username,
			database.Database, profile.CACertPathForCluster(rootCluster),
			profile.DatabaseCertPathForCluster(tc.SiteName, database.ServiceName), profile.KeyPath())
	}
	return nil
}

type dbConfigInfo struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user,omitempty"`
	Database string `json:"database,omitempty"`
	CA       string `json:"ca"`
	Cert     string `json:"cert"`
	Key      string `json:"key"`
}

func serializeDatabaseConfig(configInfo *dbConfigInfo, format string) (string, error) {
	var out []byte
	var err error
	if format == dbFormatJSON {
		out, err = utils.FastMarshalIndent(configInfo, "", "  ")
	} else {
		out, err = yaml.Marshal(configInfo)
	}
	return string(out), trace.Wrap(err)
}

// maybeStartLocalProxy starts local TLS ALPN proxy if needed depending on the
// connection scenario and returns a list of options to use in the connect
// command.
func maybeStartLocalProxy(cf *CLIConf, tc *client.TeleportClient, profile *client.ProfileStatus, db *tlsca.RouteToDatabase,
	database types.Database, cluster string,
) ([]dbcmd.ConnectCommandFunc, error) {
	// Snowflake only works in the local tunnel mode.
	localProxyTunnel := cf.LocalProxyTunnel
	if db.Protocol == defaults.ProtocolSnowflake {
		localProxyTunnel = true
	}
	// Local proxy is started if TLS routing is enabled, or if this is a SQL
	// Server connection which always requires a local proxy.
	if !tc.TLSRoutingEnabled && db.Protocol != defaults.ProtocolSQLServer {
		return []dbcmd.ConnectCommandFunc{}, nil
	}

	log.Debugf("Starting local proxy")

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	opts, err := prepareLocalProxyOptions(&localProxyConfig{
		cliConf:          cf,
		teleportClient:   tc,
		profile:          profile,
		routeToDatabase:  db,
		database:         database,
		listener:         listener,
		localProxyTunnel: localProxyTunnel,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	lp, err := mkLocalProxy(cf.Context, opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	go func() {
		defer listener.Close()
		if err := lp.Start(cf.Context); err != nil {
			log.WithError(err).Errorf("Failed to start local proxy")
		}
	}()

	addr, err := utils.ParseAddr(lp.GetAddr())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// When connecting over TLS, psql only validates hostname against presented
	// certificate's DNS names. As such, connecting to 127.0.0.1 will fail
	// validation, so connect to localhost.
	host := "localhost"
	return []dbcmd.ConnectCommandFunc{
		dbcmd.WithLocalProxy(host, addr.Port(0), profile.CACertPathForCluster(cluster)),
	}, nil
}

// localProxyConfig is an argument pack used in prepareLocalProxyOptions().
type localProxyConfig struct {
	cliConf         *CLIConf
	teleportClient  *client.TeleportClient
	profile         *client.ProfileStatus
	routeToDatabase *tlsca.RouteToDatabase
	database        types.Database
	listener        net.Listener
	// localProxyTunnel keeps the same value as cliConf.LocalProxyTunnel, but
	// it's always true for Snowflake database. Value is copied here to not modify
	// cli arguments directly.
	localProxyTunnel bool
}

// prepareLocalProxyOptions created localProxyOpts needed to create local proxy from localProxyConfig.
func prepareLocalProxyOptions(arg *localProxyConfig) (localProxyOpts, error) {
	// If user requested no client auth, open an authenticated tunnel using
	// client cert/key of the database.
	certFile := arg.cliConf.LocalProxyCertFile
	keyFile := arg.cliConf.LocalProxyKeyFile
	if certFile == "" && arg.localProxyTunnel {
		certFile = arg.profile.DatabaseCertPathForCluster(arg.cliConf.SiteName, arg.routeToDatabase.ServiceName)
		keyFile = arg.profile.KeyPath()
	}

	opts := localProxyOpts{
		proxyAddr: arg.teleportClient.WebProxyAddr,
		listener:  arg.listener,
		protocols: []common.Protocol{common.Protocol(arg.routeToDatabase.Protocol)},
		insecure:  arg.cliConf.InsecureSkipVerify,
		certFile:  certFile,
		keyFile:   keyFile,
	}

	// For SQL Server connections, local proxy must be configured with the
	// client certificate that will be used to route connections.
	if arg.routeToDatabase.Protocol == defaults.ProtocolSQLServer {
		opts.certFile = arg.profile.DatabaseCertPathForCluster("", arg.routeToDatabase.ServiceName)
		opts.keyFile = arg.profile.KeyPath()
	}

	// To set correct MySQL server version DB proxy needs additional protocol.
	if !arg.localProxyTunnel && arg.routeToDatabase.Protocol == defaults.ProtocolMySQL {
		if arg.database == nil {
			var err error
			arg.database, err = getDatabase(arg.cliConf, arg.teleportClient, arg.routeToDatabase.ServiceName)
			if err != nil {
				return localProxyOpts{}, trace.Wrap(err)
			}
		}

		mysqlServerVersionProto := mySQLVersionToProto(arg.database)
		if mysqlServerVersionProto != "" {
			opts.protocols = append(opts.protocols, common.Protocol(mysqlServerVersionProto))
		}
	}

	return opts, nil
}

// mySQLVersionToProto returns base64 encoded MySQL server version with MySQL protocol prefix.
// If version is not set in the past database an empty string is returned.
func mySQLVersionToProto(database types.Database) string {
	version := database.GetMySQLServerVersion()
	if version == "" {
		return ""
	}

	versionBase64 := base64.StdEncoding.EncodeToString([]byte(version))

	// Include MySQL server version
	return string(common.ProtocolMySQLWithVerPrefix) + versionBase64
}

// onDatabaseConnect implements "tsh db connect" command.
func onDatabaseConnect(cf *CLIConf) error {
	tc, err := makeClient(cf, false)
	if err != nil {
		return trace.Wrap(err)
	}
	profile, err := client.StatusCurrent(cf.HomePath, cf.Proxy, cf.IdentityFileIn)
	if err != nil {
		return trace.Wrap(err)
	}
	routeToDatabase, database, err := getDatabaseInfo(cf, tc, cf.DatabaseService)
	if err != nil {
		return trace.Wrap(err)
	}
	// Check is cert is still valid or DB connection requires MFA. If yes trigger db login logic.
	relogin, err := needRelogin(cf, tc, routeToDatabase, profile)
	if err != nil {
		return trace.Wrap(err)
	}
	if relogin {
		if err := databaseLogin(cf, tc, *routeToDatabase, true); err != nil {
			return trace.Wrap(err)
		}
	}

	key, err := tc.LocalAgent().GetCoreKey()
	if err != nil {
		return trace.Wrap(err)
	}
	rootClusterName, err := key.RootClusterName()
	if err != nil {
		return trace.Wrap(err)
	}

	opts, err := maybeStartLocalProxy(cf, tc, profile, routeToDatabase, database, rootClusterName)
	if err != nil {
		return trace.Wrap(err)
	}
	opts = append(opts, dbcmd.WithLogger(log))

	cmd, err := dbcmd.NewCmdBuilder(tc, profile, routeToDatabase, rootClusterName, opts...).GetConnectCommand()
	if err != nil {
		return trace.Wrap(err)
	}
	log.Debug(cmd.String())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	err = cmd.Run()
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// getDatabaseInfo fetches information about the database from tsh profile is DB is active in profile. Otherwise,
// the ListDatabases endpoint is called.
func getDatabaseInfo(cf *CLIConf, tc *client.TeleportClient, dbName string) (*tlsca.RouteToDatabase, types.Database, error) {
	database, err := pickActiveDatabase(cf)
	if err == nil {
		return database, nil, nil
	}
	if !trace.IsNotFound(err) {
		return nil, nil, trace.Wrap(err)
	}
	db, err := getDatabase(cf, tc, dbName)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return &tlsca.RouteToDatabase{
		ServiceName: db.GetName(),
		Protocol:    db.GetProtocol(),
		Username:    cf.DatabaseUser,
		Database:    cf.DatabaseName,
	}, db, nil
}

func getDatabase(cf *CLIConf, tc *client.TeleportClient, dbName string) (types.Database, error) {
	var databases []types.Database
	err := client.RetryWithRelogin(cf.Context, tc, func() error {
		allDatabases, err := tc.ListDatabases(cf.Context, &proto.ListResourcesRequest{
			Namespace:           tc.Namespace,
			PredicateExpression: fmt.Sprintf(`name == "%s"`, dbName),
		})
		// Kept for fallback in case an older auth does not apply filters.
		// DELETE IN 11.0.0
		for _, database := range allDatabases {
			if database.GetName() == dbName {
				databases = append(databases, database)
			}
		}
		return trace.Wrap(err)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(databases) == 0 {
		return nil, trace.NotFound(
			"database %q not found, use '%v' to see registered databases", dbName, formatDatabaseListCommand(cf.SiteName))
	}
	return databases[0], nil
}

func needRelogin(cf *CLIConf, tc *client.TeleportClient, database *tlsca.RouteToDatabase, profile *client.ProfileStatus) (bool, error) {
	found := false
	activeDatabases, err := profile.DatabasesForCluster(tc.SiteName)
	if err != nil {
		return false, trace.Wrap(err)
	}

	for _, v := range activeDatabases {
		if v.ServiceName == database.ServiceName {
			found = true
		}
	}
	// database not found in active list of databases.
	if !found {
		return true, nil
	}

	// For database protocols where database username is encoded in client certificate like Mongo
	// check if the command line dbUser matches the encoded username in database certificate.
	userChanged, err := dbInfoHasChanged(cf, profile.DatabaseCertPathForCluster(tc.SiteName, database.ServiceName))
	if err != nil {
		return false, trace.Wrap(err)
	}
	if userChanged {
		return true, nil
	}

	// Call API and check is a user needs to use MFA to connect to the database.
	mfaRequired, err := isMFADatabaseAccessRequired(cf, tc, database)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return mfaRequired, nil
}

// dbInfoHasChanged checks if cliConf.DatabaseUser or cliConf.DatabaseName info has changed in the user database certificate.
func dbInfoHasChanged(cf *CLIConf, certPath string) (bool, error) {
	if cf.DatabaseUser == "" && cf.DatabaseName == "" {
		return false, nil
	}

	buff, err := os.ReadFile(certPath)
	if err != nil {
		return false, trace.Wrap(err)
	}
	cert, err := tlsca.ParseCertificatePEM(buff)
	if err != nil {
		return false, trace.Wrap(err)
	}
	identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
	if err != nil {
		return false, trace.Wrap(err)
	}

	if cf.DatabaseUser != "" && cf.DatabaseUser != identity.RouteToDatabase.Username {
		log.Debugf("Will reissue database certificate for user %s (was %s)", cf.DatabaseUser, identity.RouteToDatabase.Username)
		return true, nil
	}
	if cf.DatabaseName != "" && cf.DatabaseName != identity.RouteToDatabase.Database {
		log.Debugf("Will reissue database certificate for database name %s (was %s)", cf.DatabaseName, identity.RouteToDatabase.Database)
		return true, nil
	}
	return false, nil
}

// isMFADatabaseAccessRequired calls the IsMFARequired endpoint in order to get from user roles if access to the database
// requires MFA.
func isMFADatabaseAccessRequired(cf *CLIConf, tc *client.TeleportClient, database *tlsca.RouteToDatabase) (bool, error) {
	proxy, err := tc.ConnectToProxy(cf.Context)
	if err != nil {
		return false, trace.Wrap(err)
	}
	cluster, err := proxy.ConnectToCluster(cf.Context, tc.SiteName)
	if err != nil {
		return false, trace.Wrap(err)
	}
	defer cluster.Close()

	dbParam := proto.RouteToDatabase{
		ServiceName: database.ServiceName,
		Protocol:    database.Protocol,
		Username:    database.Username,
		Database:    database.Database,
	}
	mfaResp, err := cluster.IsMFARequired(cf.Context, &proto.IsMFARequiredRequest{
		Target: &proto.IsMFARequiredRequest_Database{
			Database: &dbParam,
		},
	})
	if err != nil {
		return false, trace.Wrap(err)
	}
	return mfaResp.GetRequired(), nil
}

// pickActiveDatabase returns the database the current profile is logged into.
//
// If logged into multiple databases, returns an error unless one specified
// explicitly via --db flag.
func pickActiveDatabase(cf *CLIConf) (*tlsca.RouteToDatabase, error) {
	profile, err := client.StatusCurrent(cf.HomePath, cf.Proxy, cf.IdentityFileIn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	activeDatabases, err := profile.DatabasesForCluster(cf.SiteName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(activeDatabases) == 0 {
		return nil, trace.NotFound("Please login using 'tsh db login' first")
	}

	name := cf.DatabaseService
	if name == "" {
		if len(activeDatabases) > 1 {
			var services []string
			for _, database := range activeDatabases {
				services = append(services, database.ServiceName)
			}
			return nil, trace.BadParameter("Multiple databases are available (%v), please specify one using CLI argument",
				strings.Join(services, ", "))
		}
		name = activeDatabases[0].ServiceName
	}
	for _, db := range activeDatabases {
		if db.ServiceName == name {
			// If database user or name were provided on the CLI,
			// override the default ones.
			if cf.DatabaseUser != "" {
				db.Username = cf.DatabaseUser
			}
			if cf.DatabaseName != "" {
				db.Database = cf.DatabaseName
			}
			return &db, nil
		}
	}
	return nil, trace.NotFound("Not logged into database %q", name)
}

func formatDatabaseListCommand(clusterFlag string) string {
	if clusterFlag == "" {
		return "tsh db ls"
	}
	return fmt.Sprintf("tsh db ls --cluster=%v", clusterFlag)
}

func formatDatabaseConfigCommand(clusterFlag string, db tlsca.RouteToDatabase) string {
	if clusterFlag == "" {
		return fmt.Sprintf("tsh db config --format=cmd %v", db.ServiceName)
	}
	return fmt.Sprintf("tsh db config --cluster=%v --format=cmd %v", clusterFlag, db.ServiceName)
}

func formatDatabaseConnectMessage(clusterFlag string, db tlsca.RouteToDatabase) string {
	connectCommand := formatConnectCommand(clusterFlag, db)
	configCommand := formatDatabaseConfigCommand(clusterFlag, db)

	return fmt.Sprintf(`
Connection information for database "%v" has been saved.

You can now connect to it using the following command:

  %v

Or view the connect command for the native database CLI client:

  %v

`,
		db.ServiceName,
		utils.Color(utils.Yellow, connectCommand),
		utils.Color(utils.Yellow, configCommand))
}

const (
	// dbFormatText prints database configuration in text format.
	dbFormatText = "text"
	// dbFormatCommand prints database connection command.
	dbFormatCommand = "cmd"
	// dbFormatJSON prints database info as JSON.
	dbFormatJSON = "json"
	// dbFormatYAML prints database info as YAML.
	dbFormatYAML = "yaml"
)
