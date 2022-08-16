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
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	dbprofile "github.com/gravitational/teleport/lib/client/db"
	"github.com/gravitational/teleport/lib/client/db/postgres"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// onListDatabases implements "tsh db ls" command.
func onListDatabases(cf *CLIConf) error {
	// Retrieve profile to be able to show which databases user is logged into.
	profile, err := client.StatusCurrent(cf.HomePath, cf.Proxy)
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

	dbServers, err := proxy.GetDatabaseServers(cf.Context, tc.Namespace)
	if err != nil {
		return trace.Wrap(err)
	}
	databases := types.DatabaseServers(dbServers).ToDatabases()
	sort.Sort(types.Databases(databases))

	var roleSet services.RoleSet
	if isRoleSetRequiredForShowDatabases(cf) {
		roleSet, err = fetchRoleSetForCluster(cf.Context, profile, proxy, tc.SiteName)
		if err != nil {
			log.Debugf("Failed to fetch user roles: %v.", err)
		}
	}

	activeDatabases, err := profile.DatabasesForCluster(tc.SiteName)
	if err != nil {
		return trace.Wrap(err)
	}

	sort.Sort(types.Databases(databases))
	showDatabases(cf.Stdout(), cf.SiteName, databases, activeDatabases, roleSet, cf.Verbose)
	return nil
}

func isRoleSetRequiredForShowDatabases(cf *CLIConf) bool {
	return cf.Format == "" || teleport.Text == strings.ToLower(cf.Format)
}

func fetchRoleSetForCluster(ctx context.Context, profile *client.ProfileStatus, proxy *client.ProxyClient, clusterName string) (services.RoleSet, error) {
	cluster, err := proxy.ClusterAccessPoint(ctx, clusterName)
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

	// Print after-connect message.
	fmt.Println(formatDatabaseConnectMessage(cf.SiteName, routeToDatabase))
	return nil
}

func databaseLogin(cf *CLIConf, tc *client.TeleportClient, db tlsca.RouteToDatabase) error {
	log.Debugf("Fetching database access certificate for %s on cluster %v.", db, tc.SiteName)
	// When generating certificate for MongoDB access, database username must
	// be encoded into it. This is required to be able to tell which database
	// user to authenticate the connection as.
	if db.Protocol == defaults.ProtocolMongoDB && db.Username == "" {
		return trace.BadParameter("please provide the database user name using --db-user flag")
	}
	profile, err := client.StatusCurrent(cf.HomePath, cf.Proxy)
	if err != nil {
		return trace.Wrap(err)
	}

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

	// Refresh the profile.
	profile, err = client.StatusCurrent(cf.HomePath, cf.Proxy)
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
	profile, err := client.StatusCurrent(cf.HomePath, cf.Proxy)
	if err != nil {
		return trace.Wrap(err)
	}
	activeDatabases, err := profile.DatabasesForCluster(tc.SiteName)
	if err != nil {
		return trace.Wrap(err)
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
		if err := databaseLogout(tc, db); err != nil {
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

func databaseLogout(tc *client.TeleportClient, db tlsca.RouteToDatabase) error {
	// First remove respective connection profile.
	err := dbprofile.Delete(tc, db)
	if err != nil {
		return trace.Wrap(err)
	}
	// Then remove the certificate from the keystore.
	err = tc.LogoutDatabase(db.ServiceName)
	if err != nil {
		return trace.Wrap(err)
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
	for k, v := range env {
		fmt.Printf("export %v=%v\n", k, v)
	}
	return nil
}

// onDatabaseConfig implements "tsh db config" command.
func onDatabaseConfig(cf *CLIConf) error {
	tc, err := makeClient(cf, false)
	if err != nil {
		return trace.Wrap(err)
	}
	profile, err := client.StatusCurrent(cf.HomePath, cf.Proxy)
	if err != nil {
		return trace.Wrap(err)
	}
	database, err := pickActiveDatabase(cf)
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
	case defaults.ProtocolMongoDB:
		host, port = tc.WebProxyHostPort()
	default:
		return trace.BadParameter("unknown database protocol: %q", database)
	}
	switch cf.Format {
	case dbFormatCommand:
		cmd, err := getConnectCommand(cf, tc, profile, database)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println(cmd.Path, strings.Join(cmd.Args[1:], " "))
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
			database.Database, profile.CACertPath(),
			profile.DatabaseCertPathForCluster(tc.SiteName, database.ServiceName), profile.KeyPath())
	}
	return nil
}

func startLocalALPNSNIProxy(cf *CLIConf, tc *client.TeleportClient, databaseProtocol string) (*alpnproxy.LocalProxy, error) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	lp, err := mkLocalProxy(cf, tc.WebProxyAddr, databaseProtocol, listener)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	go func() {
		defer listener.Close()
		if err := lp.Start(cf.Context); err != nil {
			log.WithError(err).Errorf("Failed to start local proxy")
		}
	}()

	return lp, nil
}

// onDatabaseConnect implements "tsh db connect" command.
func onDatabaseConnect(cf *CLIConf) error {
	tc, err := makeClient(cf, false)
	if err != nil {
		return trace.Wrap(err)
	}
	profile, err := client.StatusCurrent(cf.HomePath, cf.Proxy)
	if err != nil {
		return trace.Wrap(err)
	}
	database, err := getDatabaseInfo(cf, tc, cf.DatabaseService)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := maybeDatabaseLogin(cf, tc, profile, database); err != nil {
		return trace.Wrap(err)
	}
	var opts []ConnectCommandFunc
	if tc.TLSRoutingEnabled {
		lp, err := startLocalALPNSNIProxy(cf, tc, database.Protocol)
		if err != nil {
			return trace.Wrap(err)
		}
		addr, err := utils.ParseAddr(lp.GetAddr())
		if err != nil {
			return trace.Wrap(err)
		}

		// When connecting over TLS, psql only validates hostname against presented certificate's
		// DNS names. As such, connecting to 127.0.0.1 will fail validation, so connect to localhost.
		host := "localhost"
		opts = append(opts, WithLocalProxy(host, addr.Port(0), profile.CACertPath()))
	}
	cmd, err := getConnectCommand(cf, tc, profile, database, opts...)
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
func getDatabaseInfo(cf *CLIConf, tc *client.TeleportClient, dbName string) (*tlsca.RouteToDatabase, error) {
	database, err := pickActiveDatabase(cf)
	if err == nil {
		return database, nil
	}
	if !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	db, err := getDatabase(cf, tc, dbName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &tlsca.RouteToDatabase{
		ServiceName: db.GetName(),
		Protocol:    db.GetProtocol(),
		Username:    cf.DatabaseUser,
		Database:    cf.DatabaseName,
	}, nil
}

func getDatabase(cf *CLIConf, tc *client.TeleportClient, dbName string) (types.Database, error) {
	var databases []types.Database
	err := client.RetryWithRelogin(cf.Context, tc, func() error {
		allDatabases, err := tc.ListDatabases(cf.Context)
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

func needDatabaseRelogin(cf *CLIConf, tc *client.TeleportClient, database *tlsca.RouteToDatabase, profile *client.ProfileStatus) (bool, error) {
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
// MFA. If yes trigger db login logic.
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

// dbInfoHasChanged checks if cf.DatabaseUser or cf.DatabaseName info has changed in the user database certificate.
func dbInfoHasChanged(cf *CLIConf, certPath string) (bool, error) {
	if cf.DatabaseUser == "" && cf.DatabaseName == "" {
		return false, nil
	}

	buff, err := ioutil.ReadFile(certPath)
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
// explicily via --db flag.
func pickActiveDatabase(cf *CLIConf) (*tlsca.RouteToDatabase, error) {
	profile, err := client.StatusCurrent(cf.HomePath, cf.Proxy)
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

type connectionCommandOpts struct {
	localProxyPort int
	localProxyHost string
	caPath         string
}

type ConnectCommandFunc func(*connectionCommandOpts)

func WithLocalProxy(host string, port int, caPath string) ConnectCommandFunc {
	return func(opts *connectionCommandOpts) {
		opts.localProxyPort = port
		opts.localProxyHost = host
		opts.caPath = caPath
	}
}

func getConnectCommand(cf *CLIConf, tc *client.TeleportClient, profile *client.ProfileStatus, db *tlsca.RouteToDatabase, opts ...ConnectCommandFunc) (*exec.Cmd, error) {
	var options connectionCommandOpts
	for _, opt := range opts {
		opt(&options)
	}

	// In TLS routing mode a local proxy is started on demand so connect to it.
	host, port := tc.DatabaseProxyHostPort(*db)
	if options.localProxyPort != 0 && options.localProxyHost != "" {
		host = options.localProxyHost
		port = options.localProxyPort
	}

	switch db.Protocol {
	case defaults.ProtocolPostgres:
		return getPostgresCommand(tc, profile, db, host, port, options), nil

	case defaults.ProtocolCockroachDB:
		return getCockroachCommand(tc, profile, db, host, port, options), nil

	case defaults.ProtocolMySQL:
		return getMySQLCommand(tc, profile, db, options), nil

	case defaults.ProtocolMongoDB:
		return getMongoCommand(tc, profile, db, host, port, options), nil
	}

	return nil, trace.BadParameter("unsupported database protocol: %v", db)
}

func getPostgresCommand(tc *client.TeleportClient, profile *client.ProfileStatus, db *tlsca.RouteToDatabase, host string, port int, options connectionCommandOpts) *exec.Cmd {
	return exec.Command(postgresBin,
		postgres.GetConnString(dbprofile.New(tc, *db, *profile, host, port)))
}

func getCockroachCommand(tc *client.TeleportClient, profile *client.ProfileStatus, db *tlsca.RouteToDatabase, host string, port int, options connectionCommandOpts) *exec.Cmd {
	// If cockroach CLI client is not available, fallback to psql.
	if _, err := exec.LookPath(cockroachBin); err != nil {
		log.Debugf("Couldn't find %q client in PATH, falling back to %q: %v.",
			cockroachBin, postgresBin, err)
		return exec.Command(postgresBin,
			postgres.GetConnString(dbprofile.New(tc, *db, *profile, host, port)))
	}
	return exec.Command(cockroachBin, "sql", "--url",
		postgres.GetConnString(dbprofile.New(tc, *db, *profile, host, port)))
}

func getMySQLCommand(tc *client.TeleportClient, profile *client.ProfileStatus, db *tlsca.RouteToDatabase, options connectionCommandOpts) *exec.Cmd {
	args := []string{fmt.Sprintf("--defaults-group-suffix=_%v-%v", tc.SiteName, db.ServiceName)}
	if db.Username != "" {
		args = append(args, "--user", db.Username)
	}
	if db.Database != "" {
		args = append(args, "--database", db.Database)
	}

	if options.localProxyPort != 0 {
		args = append(args, "--port", strconv.Itoa(options.localProxyPort))
		args = append(args, "--host", options.localProxyHost)
		// MySQL CLI treats localhost as a special value and tries to use Unix Domain Socket for connection
		// To enforce TCP connection protocol needs to be explicitly specified.
		if options.localProxyHost == "localhost" {
			args = append(args, "--protocol", "TCP")
		}
	}

	return exec.Command(mysqlBin, args...)
}

func getMongoCommand(tc *client.TeleportClient, profile *client.ProfileStatus, db *tlsca.RouteToDatabase, host string, port int, options connectionCommandOpts) *exec.Cmd {
	args := []string{
		"--host", host,
		"--port", strconv.Itoa(port),
		"--ssl",
		"--sslPEMKeyFile", profile.DatabaseCertPathForCluster(tc.SiteName, db.ServiceName),
	}

	if options.caPath != "" {
		// caPath is set only if mongo connects to the Teleport Proxy via ALPN SNI Local Proxy
		// and connection is terminated by proxy identity certificate.
		args = append(args, []string{"--sslCAFile", options.caPath}...)
	}
	if db.Database != "" {
		args = append(args, db.Database)
	}
	return exec.Command(mongoBin, args...)
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
)

const (
	// postgresBin is the Postgres client binary name.
	postgresBin = "psql"
	// cockroachBin is the Cockroach client binary name.
	cockroachBin = "cockroach"
	// mysqlBin is the MySQL client binary name.
	mysqlBin = "mysql"
	// mongoBin is the Mongo client binary name.
	mongoBin = "mongo"
)
