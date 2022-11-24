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
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"strings"
	"text/template"

	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	dbprofile "github.com/gravitational/teleport/lib/client/db"
	"github.com/gravitational/teleport/lib/client/db/dbcmd"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/srv/db/common/role"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// onListDatabases implements "tsh db ls" command.
func onListDatabases(cf *CLIConf) error {
	if cf.ListAll {
		return trace.Wrap(listDatabasesAllClusters(cf))
	}

	// Retrieve profile to be able to show which databases user is logged into.
	profile, err := client.StatusCurrent(cf.HomePath, cf.Proxy, cf.IdentityFileIn)
	if err != nil {
		return trace.Wrap(err)
	}

	tc, err := makeClient(cf, false)
	if err != nil {
		return trace.Wrap(err)
	}

	var proxy *client.ProxyClient
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		proxy, err = tc.ConnectToProxy(cf.Context)
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxy.Close()

	databases, err := proxy.FindDatabasesByFiltersForCluster(cf.Context, *tc.DefaultResourceFilter(), tc.SiteName)
	if err != nil {
		return trace.Wrap(err)
	}

	roleSet, err := fetchRoleSetForCluster(cf.Context, profile, proxy, tc.SiteName)
	if err != nil {
		log.Debugf("Failed to fetch user roles: %v.", err)
	}

	activeDatabases, err := profile.DatabasesForCluster(tc.SiteName)
	if err != nil {
		return trace.Wrap(err)
	}

	sort.Sort(types.Databases(databases))
	return trace.Wrap(showDatabases(cf.Stdout(), cf.SiteName, databases, activeDatabases, roleSet, cf.Format, cf.Verbose))
}

func fetchRoleSetForCluster(ctx context.Context, profile *client.ProfileStatus, proxy *client.ProxyClient, clusterName string) (services.RoleSet, error) {
	cluster, err := proxy.ConnectToCluster(ctx, clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer cluster.Close()

	roleSet, err := services.FetchAllClusterRoles(ctx, cluster, profile.Roles, profile.Traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return roleSet, nil
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
	// Fetch database listings for profiles in parallel. Set an arbitrary limit
	// just in case.
	group, groupCtx := errgroup.WithContext(cf.Context)
	group.SetLimit(4)

	dbListingsResultChan := make(chan databaseListings)
	dbListingsCollectChan := make(chan databaseListings)
	go func() {
		var dbListings databaseListings
		for {
			select {
			case items := <-dbListingsCollectChan:
				dbListings = append(dbListings, items...)
			case <-groupCtx.Done():
				dbListingsResultChan <- dbListings
				return
			}
		}
	}()

	err := forEachProfile(cf, func(tc *client.TeleportClient, profile *client.ProfileStatus) error {
		group.Go(func() error {
			proxy, err := tc.ConnectToProxy(groupCtx)
			if err != nil {
				return trace.Wrap(err)
			}
			defer proxy.Close()

			sites, err := proxy.GetSites(groupCtx)
			if err != nil {
				return trace.Wrap(err)
			}

			var dbListings databaseListings
			for _, site := range sites {
				databases, err := proxy.FindDatabasesByFiltersForCluster(groupCtx, *tc.DefaultResourceFilter(), site.Name)
				if err != nil {
					return trace.Wrap(err)
				}

				roleSet, err := fetchRoleSetForCluster(groupCtx, profile, proxy, site.Name)
				if err != nil {
					log.Debugf("Failed to fetch user roles: %v.", err)
				}

				for _, database := range databases {
					dbListings = append(dbListings, databaseListing{
						Proxy:    profile.ProxyURL.Host,
						Cluster:  site.Name,
						roleSet:  roleSet,
						Database: database,
					})
				}
			}

			dbListingsCollectChan <- dbListings
			return nil
		})
		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if err := group.Wait(); err != nil {
		return trace.Wrap(err)
	}

	dbListings := <-dbListingsResultChan
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
	routeToDatabase := tlsca.RouteToDatabase{
		ServiceName: cf.DatabaseService,
		Protocol:    database.GetProtocol(),
		Username:    cf.DatabaseUser,
		Database:    cf.DatabaseName,
	}

	if err := databaseLogin(cf, tc, routeToDatabase); err != nil {
		return trace.Wrap(err)
	}

	// Print after-login message.
	templateData := map[string]string{
		"name":           routeToDatabase.ServiceName,
		"connectCommand": utils.Color(utils.Yellow, formatDatabaseConnectCommand(cf.SiteName, routeToDatabase)),
	}

	if shouldUseLocalProxyForDatabase(tc, &routeToDatabase) {
		templateData["proxyCommand"] = utils.Color(utils.Yellow, formatDatabaseProxyCommand(cf.SiteName, routeToDatabase))
	} else {
		templateData["configCommand"] = utils.Color(utils.Yellow, formatDatabaseConfigCommand(cf.SiteName, routeToDatabase))
	}
	return trace.Wrap(dbConnectTemplate.Execute(cf.Stdout(), templateData))
}

// checkAndSetDBRouteDefaults checks the database route and sets defaults for certificate generation.
func checkAndSetDBRouteDefaults(r *tlsca.RouteToDatabase) error {
	// When generating certificate for MongoDB access, database username must
	// be encoded into it. This is required to be able to tell which database
	// user to authenticate the connection as.
	if r.Protocol == defaults.ProtocolMongoDB && r.Username == "" {
		return trace.BadParameter("please provide the database user name using --db-user flag")
	}
	if r.Protocol == defaults.ProtocolRedis && r.Username == "" {
		// Default to "default" in the same way as Redis does. We need the username to check access on our side.
		// ref: https://redis.io/commands/auth
		r.Username = defaults.DefaultRedisUsername
	}
	return nil
}

func databaseLogin(cf *CLIConf, tc *client.TeleportClient, db tlsca.RouteToDatabase) error {
	log.Debugf("Fetching database access certificate for %s on cluster %v.", db, tc.SiteName)
	if err := checkAndSetDBRouteDefaults(&db); err != nil {
		return trace.Wrap(err)
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
			}, nil /*applyOpts*/)
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
	return trace.Wrap(err)
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

	if !dbprofile.IsSupported(*database) {
		return trace.BadParameter(dbCmdUnsupportedDBProtocol,
			cf.CommandWithBinary(),
			defaults.ReadableDatabaseProtocol(database.Protocol),
		)
	}
	// MySQL requires ALPN local proxy in signle port mode.
	if tc.TLSRoutingEnabled && database.Protocol == defaults.ProtocolMySQL {
		return trace.BadParameter(dbCmdUnsupportedTLSRouting,
			cf.CommandWithBinary(),
			defaults.ReadableDatabaseProtocol(database.Protocol),
		)
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

	// "tsh db config" prints out instructions for native clients to connect to
	// the remote proxy directly. Return errors here when direct connection
	// does NOT work (e.g. when ALPN local proxy is required).
	if isLocalProxyAlwaysRequired(database.Protocol) {
		return trace.BadParameter(dbCmdUnsupportedDBProtocol,
			cf.CommandWithBinary(),
			defaults.ReadableDatabaseProtocol(database.Protocol),
		)
	}
	// MySQL requires ALPN local proxy in signle port mode.
	if tc.TLSRoutingEnabled && database.Protocol == defaults.ProtocolMySQL {
		return trace.BadParameter(dbCmdUnsupportedTLSRouting,
			cf.CommandWithBinary(),
			defaults.ReadableDatabaseProtocol(database.Protocol),
		)
	}

	host, port := tc.DatabaseProxyHostPort(*database)
	rootCluster, err := tc.RootClusterName(cf.Context)
	if err != nil {
		return trace.Wrap(err)
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
func maybeStartLocalProxy(ctx context.Context, cf *CLIConf, tc *client.TeleportClient, profile *client.ProfileStatus, db *tlsca.RouteToDatabase,
	database types.Database, rootClusterName string,
) ([]dbcmd.ConnectCommandFunc, error) {
	if !shouldUseLocalProxyForDatabase(tc, db) {
		return []dbcmd.ConnectCommandFunc{}, nil
	}

	// Some protocols (Snowflake, Elasticsearch) only works in the local tunnel mode.
	localProxyTunnel := cf.LocalProxyTunnel
	if db.Protocol == defaults.ProtocolSnowflake || db.Protocol == defaults.ProtocolElasticsearch {
		localProxyTunnel = true
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
		rootClusterName:  rootClusterName,
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
		if err := lp.Start(ctx); err != nil {
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
	cmdOpts := []dbcmd.ConnectCommandFunc{
		dbcmd.WithLocalProxy(host, addr.Port(0), profile.CACertPathForCluster(rootClusterName)),
	}
	if localProxyTunnel {
		cmdOpts = append(cmdOpts, dbcmd.WithNoTLS())
	}
	return cmdOpts, nil
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
	rootClusterName  string
}

// prepareLocalProxyOptions created localProxyOpts needed to create local proxy from localProxyConfig.
func prepareLocalProxyOptions(arg *localProxyConfig) (*localProxyOpts, error) {
	certFile := arg.cliConf.LocalProxyCertFile
	keyFile := arg.cliConf.LocalProxyKeyFile
	if arg.routeToDatabase.Protocol == defaults.ProtocolSQLServer ||
		arg.routeToDatabase.Protocol == defaults.ProtocolCassandra ||
		(arg.localProxyTunnel && certFile == "") {
		// For SQL Server and Cassandra connections, local proxy must be configured with the
		// client certificate that will be used to route connections.
		certFile = arg.profile.DatabaseCertPathForCluster(arg.teleportClient.SiteName, arg.routeToDatabase.ServiceName)
		keyFile = arg.profile.KeyPath()
	}
	certs, err := mkLocalProxyCerts(certFile, keyFile)
	if err != nil {
		if !arg.localProxyTunnel {
			return nil, trace.Wrap(err)
		}
		// local proxy with tunnel monitors its certs, so it's ok if a cert file can't be loaded.
		certs = nil
	}

	opts := &localProxyOpts{
		proxyAddr:               arg.teleportClient.WebProxyAddr,
		listener:                arg.listener,
		protocols:               []common.Protocol{common.Protocol(arg.routeToDatabase.Protocol)},
		insecure:                arg.cliConf.InsecureSkipVerify,
		certs:                   certs,
		alpnConnUpgradeRequired: alpnproxy.IsALPNConnUpgradeRequired(arg.teleportClient.WebProxyAddr, arg.cliConf.InsecureSkipVerify),
	}

	// If ALPN connection upgrade is required, explicitly use the profile CAs
	// since the tunneled TLS routing connection serves the Host cert.
	if opts.alpnConnUpgradeRequired {
		profileCAs, err := utils.NewCertPoolFromPath(arg.profile.CACertPathForCluster(arg.rootClusterName))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		opts.rootCAs = profileCAs
	}

	if arg.localProxyTunnel {
		dbRoute := *arg.routeToDatabase
		if err := checkAndSetDBRouteDefaults(&dbRoute); err != nil {
			return nil, trace.Wrap(err)
		}
		opts.middleware = client.NewDBCertChecker(arg.teleportClient, dbRoute, nil)
	}

	// To set correct MySQL server version DB proxy needs additional protocol.
	if !arg.localProxyTunnel && arg.routeToDatabase.Protocol == defaults.ProtocolMySQL {
		if arg.database == nil {
			var err error
			arg.database, err = getDatabase(arg.cliConf, arg.teleportClient, arg.routeToDatabase.ServiceName)
			if err != nil {
				return nil, trace.Wrap(err)
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
	if err := maybeDatabaseLogin(cf, tc, profile, routeToDatabase); err != nil {
		return trace.Wrap(err)
	}

	rootClusterName, err := tc.RootClusterName(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}

	// To avoid termination of background DB teleport proxy when a SIGINT is received don't use the cf.Context.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts, err := maybeStartLocalProxy(ctx, cf, tc, profile, routeToDatabase, database, rootClusterName)
	if err != nil {
		return trace.Wrap(err)
	}
	opts = append(opts, dbcmd.WithLogger(log))

	if opts, err = maybeAddDBUserPassword(database, opts); err != nil {
		return trace.Wrap(err)
	}

	bb := dbcmd.NewCmdBuilder(tc, profile, routeToDatabase, rootClusterName, opts...)
	cmd, err := bb.GetConnectCommand()
	if err != nil {
		return trace.Wrap(err)
	}
	log.Debug(cmd.String())

	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin

	// Use io.MultiWriter to duplicate stderr to the capture writer. The
	// captured stderr can be used for diagnosing command failures. The capture
	// writer captures up to a fixed number to limit memory usage.
	peakStderr := utils.NewCaptureNBytesWriter(dbcmd.PeakStderrSize)
	cmd.Stderr = io.MultiWriter(os.Stderr, peakStderr)

	err = cmd.Run()
	if err != nil {
		return dbcmd.ConvertCommandError(cmd, err, string(peakStderr.Bytes()))
	}
	return nil
}

// getDatabaseInfo fetches information about the database from tsh profile is DB is active in profile. Otherwise,
// the ListDatabases endpoint is called.
func getDatabaseInfo(cf *CLIConf, tc *client.TeleportClient, dbName string) (*tlsca.RouteToDatabase, types.Database, error) {
	database, err := pickActiveDatabase(cf)
	if err == nil {
		switch database.Protocol {
		case defaults.ProtocolCassandra:
			// Cassandra CLI connection require database resource to determine
			// if the target database is AWS hosted in order to skip the password prompt.
		default:
			return database, nil, nil
		}
	}
	if err != nil && !trace.IsNotFound(err) {
		return nil, nil, trace.Wrap(err)
	}
	db, err := getDatabase(cf, tc, dbName)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	username := cf.DatabaseUser
	databaseName := cf.DatabaseName
	if database != nil {
		if username == "" {
			username = database.Username
		}
		if databaseName == "" {
			databaseName = database.Database
		}
	}

	return &tlsca.RouteToDatabase{
		ServiceName: db.GetName(),
		Protocol:    db.GetProtocol(),
		Username:    username,
		Database:    databaseName,
	}, db, nil
}

func getDatabase(cf *CLIConf, tc *client.TeleportClient, dbName string) (types.Database, error) {
	var databases []types.Database
	err := client.RetryWithRelogin(cf.Context, tc, func() error {
		var err error
		databases, err = tc.ListDatabases(cf.Context, &proto.ListResourcesRequest{
			Namespace:           tc.Namespace,
			PredicateExpression: fmt.Sprintf(`name == "%s"`, dbName),
		})
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

func needDatabaseRelogin(cf *CLIConf, tc *client.TeleportClient, database *tlsca.RouteToDatabase, profile *client.ProfileStatus) (bool, error) {
	if cf.LocalProxyTunnel {
		// Don't login to database here if local proxy tunnel is enabled.
		// When local proxy tunnel is enabled, the local proxy will check if DB login is needed when
		// it starts and on each new connection.
		return false, nil
	}

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

// maybeDatabaseLogin checks if cert is still valid or DB connection requires
// MFA, and that client is not requesting an authenticated local proxy tunnel. If yes trigger db login logic.
func maybeDatabaseLogin(cf *CLIConf, tc *client.TeleportClient, profile *client.ProfileStatus, db *tlsca.RouteToDatabase) error {
	reloginNeeded, err := needDatabaseRelogin(cf, tc, db, profile)
	if err != nil {
		return trace.Wrap(err)
	}

	if reloginNeeded {
		return trace.Wrap(databaseLogin(cf, tc, *db))
	}
	return nil
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

// formatDatabaseConnectCommand formats an appropriate database connection
// command for a user based on the provided database parameters.
func formatDatabaseConnectCommand(clusterFlag string, active tlsca.RouteToDatabase) string {
	cmdTokens := append(
		[]string{"tsh", "db", "connect"},
		formatDatabaseConnectArgs(clusterFlag, active)...,
	)
	return strings.Join(cmdTokens, " ")
}

// formatDatabaseConnectArgs generates the arguments for "tsh db connect" command.
func formatDatabaseConnectArgs(clusterFlag string, active tlsca.RouteToDatabase) (flags []string) {
	// figure out if we need --db-user and --db-name
	matchers := role.DatabaseRoleMatchers(active.Protocol, active.Username, active.Database)
	needUser := false
	needDatabase := false

	for _, matcher := range matchers {
		_, userMatcher := matcher.(*services.DatabaseUserMatcher)
		needUser = needUser || userMatcher

		_, nameMatcher := matcher.(*services.DatabaseNameMatcher)
		needDatabase = needDatabase || nameMatcher
	}

	if clusterFlag != "" {
		flags = append(flags, fmt.Sprintf("--cluster=%s", clusterFlag))
	}
	if active.Username == "" && needUser {
		flags = append(flags, "--db-user=<user>")
	}
	if active.Database == "" && needDatabase {
		flags = append(flags, "--db-name=<name>")
	}
	flags = append(flags, active.ServiceName)
	return
}

// formatDatabaseProxyCommand formats the "tsh proxy db" command.
func formatDatabaseProxyCommand(clusterFlag string, active tlsca.RouteToDatabase) string {
	cmdTokens := append(
		// "--tunnel" mode is more user friendly and supports all DB protocols.
		[]string{"tsh", "proxy", "db", "--tunnel"},
		// Rest of the args are the same as "tsh db connect".
		formatDatabaseConnectArgs(clusterFlag, active)...,
	)
	return strings.Join(cmdTokens, " ")
}

// formatDatabaseConfigCommand formats the "tsh db config" command.
func formatDatabaseConfigCommand(clusterFlag string, db tlsca.RouteToDatabase) string {
	if clusterFlag == "" {
		return fmt.Sprintf("tsh db config --format=cmd %v", db.ServiceName)
	}
	return fmt.Sprintf("tsh db config --cluster=%v --format=cmd %v", clusterFlag, db.ServiceName)
}

// shouldUseLocalProxyForDatabase returns true if the ALPN local proxy should
// be used for connecting to the provided database.
func shouldUseLocalProxyForDatabase(tc *client.TeleportClient, db *tlsca.RouteToDatabase) bool {
	return tc.TLSRoutingEnabled || isLocalProxyAlwaysRequired(db.Protocol)
}

// isLocalProxyAlwaysRequired returns true for protocols that always requires
// an ALPN local proxy.
func isLocalProxyAlwaysRequired(protocol string) bool {
	switch protocol {
	case defaults.ProtocolSQLServer,
		defaults.ProtocolSnowflake,
		defaults.ProtocolCassandra:
		return true
	default:
		return false
	}
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

const (
	// dbCmdUnsupportedTLSRouting is the error message printed when some
	// database subcommands are not supported because TLS routing is enabled.
	dbCmdUnsupportedTLSRouting = `"%v" is not supported for %v databases when TLS routing is enabled on the Teleport Proxy Service.

Please use "tsh db connect" or "tsh proxy db" to connect to the database.`

	// dbCmdUnsupportedDBProtocol is the error message printed when some
	// database subcommands are run against unsupported database protocols.
	dbCmdUnsupportedDBProtocol = `"%v" is not supported for %v databases.

Please use "tsh db connect" or "tsh proxy db" to connect to the database.`
)

var (
	// dbConnectTemplate is the message printed after a successful "tsh db login" on how to connect.
	dbConnectTemplate = template.Must(template.New("").Parse(`Connection information for database "{{ .name }}" has been saved.

You can now connect to it using the following command:

  {{.connectCommand}}

{{if .configCommand -}}
Or view the connect command for the native database CLI client:

  {{ .configCommand }}

{{end -}}
{{if .proxyCommand -}}
Or start a local proxy for database GUI clients:

  {{ .proxyCommand }}

{{end -}}
`))
)
