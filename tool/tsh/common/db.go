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

package common

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/client"
	dbprofile "github.com/gravitational/teleport/lib/client/db"
	"github.com/gravitational/teleport/lib/client/db/dbcmd"
	"github.com/gravitational/teleport/lib/client/db/oracle"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/srv/db/common/role"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// onListDatabases implements "tsh db ls" command.
func onListDatabases(cf *CLIConf) error {
	if cf.ListAll {
		return trace.Wrap(listDatabasesAllClusters(cf))
	}

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	profile, err := tc.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}

	var clusterClient *client.ClusterClient
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err = tc.ConnectToCluster(cf.Context)
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer clusterClient.Close()

	servers, err := apiclient.GetAllResources[types.DatabaseServer](cf.Context, clusterClient.AuthClient, tc.ResourceFilter(types.KindDatabaseServer))
	if err != nil {
		return trace.Wrap(err)
	}

	accessChecker, err := services.NewAccessCheckerForRemoteCluster(cf.Context, profile.AccessInfo(), tc.SiteName, clusterClient.AuthClient)
	if err != nil {
		logger.DebugContext(cf.Context, "Failed to fetch user roles", "error", err)
	}

	activeDatabases, err := profile.DatabasesForCluster(tc.SiteName)
	if err != nil {
		return trace.Wrap(err)
	}

	databases := types.DatabaseServers(servers).ToDatabases()
	sort.Sort(types.Databases(databases))
	return trace.Wrap(showDatabases(cf, databases, activeDatabases, accessChecker))
}

type databaseListing struct {
	Proxy         string                 `json:"proxy"`
	Cluster       string                 `json:"cluster"`
	accessChecker services.AccessChecker `json:"-"`
	Database      types.Database         `json:"database"`
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
	tracer := cf.TracingProvider.Tracer(teleport.ComponentTSH)
	clusters, err := getClusterClients(cf, types.KindDatabaseServer)
	if err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		// close all clients
		for _, cluster := range clusters {
			_ = cluster.Close()
		}
	}()

	// Fetch database listings for all clusters in parallel with an upper limit
	group, groupCtx := errgroup.WithContext(cf.Context)
	group.SetLimit(10)

	// mu guards access to dbListings
	var (
		mu         sync.Mutex
		dbListings databaseListings
		errors     []error
	)
	for _, cluster := range clusters {
		cluster := cluster
		if cluster.connectionError != nil {
			mu.Lock()
			errors = append(errors, cluster.connectionError)
			mu.Unlock()
			continue
		}

		group.Go(func() error {
			ctx, span := tracer.Start(
				groupCtx,
				"ListDatabases",
				oteltrace.WithAttributes(attribute.String("cluster", cluster.name)))
			defer span.End()

			logger := logger.With("cluster", cluster.name)
			databases, err := apiclient.GetAllResources[types.DatabaseServer](ctx, cluster.auth, &cluster.req)
			if err != nil {
				logger.ErrorContext(ctx, "Failed to get databases", "error", err)

				mu.Lock()
				errors = append(errors, trace.ConnectionProblem(err, "failed to list databases for cluster %s: %v", cluster.name, err))
				mu.Unlock()
				return nil
			}

			accessChecker, err := services.NewAccessCheckerForRemoteCluster(ctx, cluster.profile.AccessInfo(), cluster.name, cluster.auth)
			if err != nil {
				logger.DebugContext(ctx, "Failed to fetch user roles", "error", err)
			}

			localDBListings := make(databaseListings, 0, len(databases))
			for _, database := range types.DatabaseServers(databases).ToDatabases() {
				localDBListings = append(localDBListings, databaseListing{
					Proxy:         cluster.profile.ProxyURL.Host,
					Cluster:       cluster.name,
					accessChecker: accessChecker,
					Database:      database,
				})
			}
			mu.Lock()
			dbListings = append(dbListings, localDBListings...)
			mu.Unlock()

			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return trace.Wrap(err)
	}

	if len(dbListings) == 0 && len(errors) > 0 {
		return trace.NewAggregate(errors...)
	}

	sort.Sort(dbListings)

	profile, err := cf.ProfileStatus()
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
		printDatabasesWithClusters(cf, dbListings, active)
	case teleport.JSON, teleport.YAML:
		out, err := serializeDatabasesAllClusters(dbListings, format)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Fprintln(cf.Stdout(), out)
	default:
		return trace.BadParameter("unsupported format %q", format)
	}

	return trace.NewAggregate(errors...)
}

// onDatabaseLogin implements "tsh db login" command.
func onDatabaseLogin(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	profile, err := tc.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}
	routes, err := profile.DatabasesForCluster(tc.SiteName)
	if err != nil {
		return trace.Wrap(err)
	}
	dbInfo, err := getDatabaseInfo(cf, tc, routes)
	if err != nil {
		return trace.Wrap(err)
	}

	database, err := dbInfo.GetDatabase(cf.Context, tc)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := databaseLogin(cf, tc, dbInfo); err != nil {
		return trace.Wrap(err)
	}

	// Print after-login message.
	templateData := map[string]string{
		"name": dbInfo.ServiceName,
	}

	if protocolSupportsInteractiveMode(database.GetProtocol()) {
		templateData["connectCommand"] = utils.Color(utils.Yellow, formatDatabaseConnectCommand(cf.SiteName, dbInfo.RouteToDatabase))
	}

	requires := getDBLocalProxyRequirement(tc, dbInfo.RouteToDatabase)
	if requires.localProxy {
		templateData["proxyCommand"] = utils.Color(utils.Yellow, formatDatabaseProxyCommand(cf.SiteName, dbInfo.RouteToDatabase))
	} else {
		templateData["configCommand"] = utils.Color(utils.Yellow, formatDatabaseConfigCommand(cf.SiteName, dbInfo.RouteToDatabase))
	}
	return trace.Wrap(dbConnectTemplate.Execute(cf.Stdout(), templateData))
}

// protocolSupportsInteractiveMode checks if DB Protocol integration support
// client interactive mode that is needed for the tsh db connect flow.
func protocolSupportsInteractiveMode(dbProtocol string) bool {
	switch dbProtocol {
	case defaults.ProtocolDynamoDB, defaults.ProtocolClickHouseHTTP:
		return false
	}
	return true
}

func databaseLogin(cf *CLIConf, tc *client.TeleportClient, dbInfo *databaseInfo) error {
	logger.DebugContext(cf.Context, "Fetching database access certificate",
		"database", dbInfo.RouteToDatabase,
		"cluster", tc.SiteName,
	)

	profile, err := tc.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}

	var keyRing *client.KeyRing
	// Identity files themselves act as the database credentials (if any), so
	// don't bother fetching new certs.
	if profile.IsVirtual {
		logger.InfoContext(cf.Context, "Note: already logged in due to an identity file (`-i ...`); will only update database config files")
	} else {
		if err = client.RetryWithRelogin(cf.Context, tc, func() error {
			keyRing, err = tc.IssueUserCertsWithMFA(cf.Context, client.ReissueParams{
				RouteToCluster: tc.SiteName,
				RouteToDatabase: proto.RouteToDatabase{
					ServiceName: dbInfo.ServiceName,
					Protocol:    dbInfo.Protocol,
					Username:    dbInfo.Username,
					Database:    dbInfo.Database,
					Roles:       dbInfo.Roles,
				},
				AccessRequests: profile.ActiveRequests.AccessRequests,
			})
			return trace.Wrap(err)
		}); err != nil {
			return trace.Wrap(err)
		}

		if err = tc.LocalAgent().AddDatabaseKeyRing(keyRing); err != nil {
			return trace.Wrap(err)
		}
	}

	if dbInfo.Protocol == defaults.ProtocolOracle {
		if err := generateDBLocalProxyCert(keyRing.TLSPrivateKey, profile); err != nil {
			return trace.Wrap(err)
		}
		err = oracle.GenerateClientConfiguration(keyRing.TLSPrivateKey, dbInfo.RouteToDatabase, profile, tc.SiteName)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// Refresh the profile.
	profile, err = tc.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}

	if len(dbInfo.Roles) > 0 {
		if route, ok := findActiveDatabase(dbInfo.ServiceName, profile.Databases); ok && len(route.Roles) == 0 {
			fmt.Fprintf(cf.Stdout(), "Warning. Selected db roles %v are not issued by the Teleport Cluster and all assigned database roles will be used instead. This is likely due to your Teleport Cluster running an older version that doesn't support this feature.\n\n", dbInfo.Roles)
		}
	}

	// Update the database-specific connection profile file.
	err = dbprofile.Add(cf.Context, tc, dbInfo.RouteToDatabase, *profile)
	return trace.Wrap(err)
}

// onDatabaseLogout implements "tsh db logout" command.
func onDatabaseLogout(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	profile, err := tc.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}
	activeRoutes, err := profile.DatabasesForCluster(tc.SiteName)
	if err != nil {
		return trace.Wrap(err)
	}
	databases, err := getDatabasesForLogout(cf, tc, activeRoutes)
	if err != nil {
		return trace.Wrap(err)
	}

	if profile.IsVirtual {
		logger.InfoContext(cf.Context, "Note: an identity file is in use (`-i ...`); will only update database config files.")
	}

	for _, db := range databases {
		if err := databaseLogout(tc, db, profile.IsVirtual); err != nil {
			return trace.Wrap(err)
		}
	}
	msg, err := makeLogoutMessage(cf, databases, activeRoutes)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintln(cf.Stdout(), msg)
	return nil
}

// makeLogoutMessage is a helper func that returns a logout message for the
// result of "tsh db logout".
func makeLogoutMessage(cf *CLIConf, logout, activeRoutes []tlsca.RouteToDatabase) (string, error) {
	switch len(logout) {
	case 1:
		return fmt.Sprintf("Logged out of database %v", logout[0].ServiceName), nil
	case len(activeRoutes):
		return "Logged out of all databases", nil
	case 0:
		selectors := newDatabaseResourceSelectors(cf)
		if selectors.IsEmpty() {
			return "", trace.NotFound("Not logged into any databases")
		}
		return "", trace.NotFound("Not logged into %s", selectors)
	default:
		names := make([]string, 0, len(logout))
		for _, route := range logout {
			names = append(names, route.ServiceName)
		}
		slices.Sort(names)
		nameLines := strings.Join(names, "\n")
		return fmt.Sprintf("Logged out of databases:\n%v", nameLines), nil
	}
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
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	profile, err := tc.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}
	routes, err := profile.DatabasesForCluster(tc.SiteName)
	if err != nil {
		return trace.Wrap(err)
	}
	database, err := pickActiveDatabase(cf, tc, routes)
	if err != nil {
		return trace.Wrap(err)
	}

	if !dbprofile.IsSupported(*database) {
		return trace.BadParameter(formatDbCmdUnsupportedDBProtocol(cf, *database))
	}
	requires := getDBLocalProxyRequirement(tc, *database)
	if requires.localProxy {
		return trace.BadParameter(formatDbCmdUnsupported(cf, *database, requires.localProxyReasons...))
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
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	profile, err := tc.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}
	routes, err := profile.DatabasesForCluster(tc.SiteName)
	if err != nil {
		return trace.Wrap(err)
	}
	database, err := pickActiveDatabase(cf, tc, routes)
	if err != nil {
		return trace.Wrap(err)
	}

	requires := getDBLocalProxyRequirement(tc, *database)
	// "tsh db config" prints out instructions for native clients to connect to
	// the remote proxy directly. Return errors here when direct connection
	// does NOT work (e.g. when ALPN local proxy is required).
	if requires.localProxy {
		msg := formatDbCmdUnsupported(cf, *database, requires.localProxyReasons...)
		return trace.BadParameter(msg)
	}

	host, port := tc.DatabaseProxyHostPort(*database)
	rootCluster, err := tc.RootClusterName(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}

	format := strings.ToLower(cf.Format)
	switch format {
	case dbFormatCommand:
		cmd, err := dbcmd.NewCmdBuilder(tc, profile, *database, rootCluster,
			dbcmd.WithPrintFormat(),
			dbcmd.WithLogger(logger),
			dbcmd.WithGetDatabaseFunc(getDatabase),
		).GetConnectCommand(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println(strings.Join(cmd.Env, " "), cmd.Path, strings.Join(cmd.Args[1:], " "))
	case dbFormatJSON, dbFormatYAML:
		configInfo := &dbConfigInfo{
			database.ServiceName, host, port, database.Username,
			database.Database, profile.CACertPathForCluster(rootCluster),
			profile.DatabaseCertPathForCluster(tc.SiteName, database.ServiceName),
			profile.DatabaseKeyPathForCluster(tc.SiteName, database.ServiceName),
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
			profile.DatabaseCertPathForCluster(tc.SiteName, database.ServiceName),
			profile.DatabaseKeyPathForCluster(tc.SiteName, database.ServiceName),
		)
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
func maybeStartLocalProxy(ctx context.Context, cf *CLIConf,
	tc *client.TeleportClient, profile *client.ProfileStatus,
	dbInfo *databaseInfo, rootClusterName string,
	requires *dbLocalProxyRequirement,
) ([]dbcmd.ConnectCommandFunc, error) {
	if !requires.localProxy {
		return nil, nil
	}
	if requires.tunnel {
		logger.DebugContext(ctx, "Starting local proxy tunnel", "reasons", requires.tunnelReasons)
	} else {
		logger.DebugContext(ctx, "Starting local proxy", "reasons", requires.localProxyReasons)
	}

	listener, err := createLocalProxyListener("localhost:0", dbInfo.RouteToDatabase, profile)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	opts, err := prepareLocalProxyOptions(&localProxyConfig{
		cf:      cf,
		tc:      tc,
		profile: profile,
		dbInfo:  dbInfo,
		tunnel:  requires.tunnel,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	lp, err := alpnproxy.NewLocalProxy(makeBasicLocalProxyConfig(cf.Context, tc, listener, cf.InsecureSkipVerify), opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	go func() {
		defer listener.Close()
		if err := lp.Start(ctx); err != nil {
			logger.ErrorContext(cf.Context, "Failed to start local proxy", "error", err)
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
	if requires.tunnel {
		cmdOpts = append(cmdOpts, dbcmd.WithNoTLS())
	}
	return cmdOpts, nil
}

// localProxyConfig is an argument pack used in prepareLocalProxyOptions().
type localProxyConfig struct {
	cf      *CLIConf
	tc      *client.TeleportClient
	profile *client.ProfileStatus
	dbInfo  *databaseInfo
	// tunnel controls whether client certs will always be used to dial upstream
	// by the local proxy, and whether db certs will be auto-reissued for the
	// connection.
	// - when `tsh db connect` needs to tunnel it will set this field.
	// - when `tsh proxy db` is used with `--tunnel` cli flag it will set this field.
	tunnel bool
}

func createLocalProxyListener(addr string, route tlsca.RouteToDatabase, profile *client.ProfileStatus) (net.Listener, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if route.Protocol == defaults.ProtocolOracle {
		localCert, err := tls.LoadX509KeyPair(
			profile.DatabaseLocalCAPath(),
			profile.TLSKeyPath(),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		config := &tls.Config{
			Certificates: []tls.Certificate{localCert},
			ServerName:   "localhost",
		}

		l = NewTLSMuxListener(l, config)
	}

	return l, nil
}

// prepareLocalProxyOptions created localProxyOpts needed to create local proxy from localProxyConfig.
func prepareLocalProxyOptions(arg *localProxyConfig) ([]alpnproxy.LocalProxyConfigOpt, error) {
	opts := []alpnproxy.LocalProxyConfigOpt{
		alpnproxy.WithDatabaseProtocol(arg.dbInfo.Protocol),
		alpnproxy.WithClusterCAsIfConnUpgrade(arg.cf.Context, arg.tc.RootClusterCACertPool),
	}

	if arg.tunnel {
		cc := client.NewDBCertChecker(arg.tc, arg.dbInfo.RouteToDatabase, nil, client.WithTTL(time.Duration(arg.cf.MinsToLive)*time.Minute))
		opts = append(opts, alpnproxy.WithMiddleware(cc))
		// When using a tunnel, try to load certs, but if that fails
		// just skip them and let the reissuer fetch new certs when the local
		// proxy starts instead.
		cert, err := loadDBCertificate(arg.tc, arg.dbInfo.ServiceName)
		if err == nil {
			cc.SetCert(cert)
		}
		return opts, nil
	}

	// no tunnel, check for protocol-specific cases
	switch arg.dbInfo.Protocol {
	case defaults.ProtocolPostgres, defaults.ProtocolCockroachDB:
		// certs are needed for non-tunnel postgres cancel requests.
		cert, err := loadDBCertificate(arg.tc, arg.dbInfo.ServiceName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		opts = append(opts, alpnproxy.WithClientCert(cert))
		opts = append(opts, alpnproxy.WithCheckCertNeeded())
	case defaults.ProtocolMySQL:
		// To set correct MySQL server version DB proxy needs additional protocol.
		db, err := arg.dbInfo.GetDatabase(arg.cf.Context, arg.tc)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		opts = append(opts, alpnproxy.WithMySQLVersionProto(db))
	}
	return opts, nil
}

// onDatabaseConnect implements "tsh db connect" command.
func onDatabaseConnect(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	profile, err := tc.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}
	routes, err := profile.DatabasesForCluster(tc.SiteName)
	if err != nil {
		return trace.Wrap(err)
	}
	dbInfo, err := getDatabaseInfo(cf, tc, routes)
	if err != nil {
		return trace.Wrap(err)
	}

	switch dbInfo.Protocol {
	case defaults.ProtocolDynamoDB, defaults.ProtocolClickHouseHTTP:
		return trace.BadParameter(formatDbCmdUnsupportedDBProtocol(cf, dbInfo.RouteToDatabase))
	}

	requires := getDBConnectLocalProxyRequirement(cf.Context, tc, dbInfo.RouteToDatabase, cf.LocalProxyTunnel)
	if err := maybeDatabaseLogin(cf, tc, profile, dbInfo, requires); err != nil {
		return trace.Wrap(err)
	}

	rootClusterName, err := tc.RootClusterName(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}

	// To avoid termination of background DB teleport proxy when a SIGINT is received don't use the cf.Context.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts, err := maybeStartLocalProxy(ctx, cf, tc, profile, dbInfo, rootClusterName, requires)
	if err != nil {
		return trace.Wrap(err)
	}
	opts = append(opts,
		dbcmd.WithLogger(logger),
		dbcmd.WithGetDatabaseFunc(dbInfo.getDatabaseForDBCmd),
	)

	if opts, err = maybeAddDBUserPassword(cf, tc, dbInfo, opts); err != nil {
		return trace.Wrap(err)
	}
	if opts, err = maybeAddGCPMetadata(cf.Context, tc, dbInfo, opts); err != nil {
		return trace.Wrap(err)
	}
	opts = maybeAddOracleOptions(cf.Context, tc, dbInfo, opts)

	bb := dbcmd.NewCmdBuilder(tc, profile, dbInfo.RouteToDatabase, rootClusterName, opts...)
	cmd, err := bb.GetConnectCommand(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}
	logger.DebugContext(ctx, "executing command", "command", logutils.StringerAttr(cmd))

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

// getDatabaseInfo fetches information about the database from tsh profile if DB
// is active in profile and no labels or predicate query are given.
// Otherwise, the ListDatabases endpoint is called.
func getDatabaseInfo(cf *CLIConf, tc *client.TeleportClient, routes []tlsca.RouteToDatabase) (*databaseInfo, error) {
	if route, err := maybePickActiveDatabase(cf, routes); err == nil && route != nil {
		info := &databaseInfo{RouteToDatabase: *route, isActive: true}
		return info, info.checkAndSetDefaults(cf, tc)
	} else if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.BadParameter("please specify a database service by name, --labels, or --query")
		}
		return nil, trace.Wrap(err)
	}

	db, err := getDatabaseByNameOrDiscoveredName(cf, tc, routes)
	switch {
	// If the database cannot be found, try again with UseSearchAsRoles. If
	// the database is then found with UseSearchAsRoles, make an access request
	// for it and elevate the user with the request ID upon approval.
	//
	// Note that the access request must be made before the database connection
	// is made to avoid mangling the request with the database client tools.
	// Thus the flow for auto database access request is different from SSH.
	//
	// Performance considerations:
	// - For common scenarios where UseSearchAsRoles is not desired, it would
	//   be rare that cf.DatabaseName would be not found in the first API call
	//   so there won't be a second call usually.
	// - accessChecker.GetAllowedSearchAsRoles can be checked to avoid the
	//   second API call but creating the access checker requires more calls.
	// - The db commands do provide "--disable-access-request" to bypass the
	//   second call. If needed, we can add it to `tsh login` and profile yaml
	//   in the future.
	case shouldRetryGetDatabaseUsingSearchAsRoles(cf, tc, err):
		orgErr := err
		if db, err = getDatabaseByNameOrDiscoveredNameUsingSearchAsRoles(cf, tc); err != nil {
			return nil, trace.Wrap(orgErr) // Returns the original not found error.
		}
		if err := makeDatabaseAccessRequestAndWaitForApproval(cf, tc, db); err != nil {
			return nil, trace.Wrap(err)
		}

		// Reset routes. Once access requeset is approved, user certs are
		// reissued with client.CertCacheDrop.
		routes = nil

	case err != nil:
		return nil, trace.Wrap(err)
	}

	info := &databaseInfo{
		database: db,
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: db.GetName(),
			Protocol:    db.GetProtocol(),
		},
	}
	// check for an active route now that we have the full db name.
	if route, ok := findActiveDatabase(db.GetName(), routes); ok {
		info.RouteToDatabase = route
		info.isActive = true
	}
	if err := info.checkAndSetDefaults(cf, tc); err != nil {
		return nil, trace.Wrap(err)
	}
	return info, nil
}

var dbCommandsWithAccessRequestSupport = []string{
	"db login",
	"proxy db",
	"db connect",
}

func shouldRetryGetDatabaseUsingSearchAsRoles(cf *CLIConf, tc *client.TeleportClient, getDatabaseError error) bool {
	// If already using SearchAsRoles, nothing to retry.
	if tc.UseSearchAsRoles {
		return false
	}
	// Only retry when the database cannot be found.
	if !trace.IsNotFound(getDatabaseError) {
		return false
	}
	// Check if auto access request is disabled.
	if cf.disableAccessRequest {
		return false
	}
	// Check if the `tsh` command supports auto access request.
	return slices.Contains(dbCommandsWithAccessRequestSupport, cf.command)
}

func makeAccessRequestForDatabase(tc *client.TeleportClient, db types.Database) (types.AccessRequest, error) {
	requestResourceIDs := []types.ResourceID{{
		ClusterName: tc.SiteName,
		Kind:        types.KindDatabase,
		Name:        db.GetName(),
	}}

	req, err := services.NewAccessRequestWithResources(tc.Username, nil /* roles */, requestResourceIDs)
	return req, trace.Wrap(err)
}

func makeDatabaseAccessRequestAndWaitForApproval(cf *CLIConf, tc *client.TeleportClient, db types.Database) error {
	req, err := makeAccessRequestForDatabase(tc, db)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintf(cf.Stdout(), "You do not currently have access to %q, attempting to request access.\n\n", db.GetName())
	if err := setAccessRequestReason(cf, req); err != nil {
		return trace.Wrap(err)
	}
	if err := sendAccessRequestAndWaitForApproval(cf, tc, req); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func requestedDatabaseRoles(cf *CLIConf) []string {
	if cf.DatabaseRoles == "" {
		return nil
	}
	return strings.Split(cf.DatabaseRoles, ",")
}

// checkAndSetDefaults checks the db route, applies cli flags, and sets defaults.
func (d *databaseInfo) checkAndSetDefaults(cf *CLIConf, tc *client.TeleportClient) error {
	if d.ServiceName == "" {
		return trace.BadParameter("missing database service name")
	}
	if cf.DatabaseUser != "" {
		d.Username = cf.DatabaseUser
	}
	if cf.DatabaseName != "" {
		d.Database = cf.DatabaseName
	}
	if dbRoles := requestedDatabaseRoles(cf); len(dbRoles) > 0 {
		d.Roles = dbRoles
	}
	db, err := d.GetDatabase(cf.Context, tc)
	if err != nil {
		if d.isActive && trace.IsNotFound(err) && strings.Contains(err.Error(), d.ServiceName) {
			hint := formatStaleDBCert(cf.SiteName, d.ServiceName)
			return trace.Wrap(err, hint)
		}
		return trace.Wrap(err)
	}
	// ensure the route protocol matches the db.
	d.Protocol = db.GetProtocol()

	needDBUser := d.Username == "" && isDatabaseUserRequired(d.Protocol)
	needDBName := d.Database == "" && isDatabaseNameRequired(d.Protocol)
	if !needDBUser && !needDBName {
		return nil
	}

	var clusterClient *client.ClusterClient
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err = tc.ConnectToCluster(cf.Context)
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer clusterClient.Close()

	profile, err := tc.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}

	checker, err := services.NewAccessCheckerForRemoteCluster(cf.Context, profile.AccessInfo(), tc.SiteName, clusterClient.AuthClient)
	if err != nil {
		return trace.Wrap(err)
	}

	if needDBUser {
		dbUser, err := getDefaultDBUser(db, checker)
		if err != nil {
			return trace.Wrap(err)
		}
		logger.DebugContext(cf.Context, "Defaulting to the allowed database user", "database_user", dbUser)
		d.Username = dbUser
	}
	if needDBName {
		dbName, err := getDefaultDBName(db, checker)
		if err != nil {
			return trace.Wrap(err)
		}
		logger.DebugContext(cf.Context, "Defaulting to the allowed database name", "database_name", dbName)
		d.Database = dbName
	}
	return nil
}

// databaseInfo wraps a RouteToDatabase and the corresponding database.
// Its purpose is to prevent repeated fetches of the same database, by lazily
// fetching and caching the database for use as needed.
type databaseInfo struct {
	tlsca.RouteToDatabase
	// database corresponds to the db route and may be nil, so use GetDatabase
	// instead of accessing it directly.
	database types.Database
	// isActive indicates an active database matched this db info.
	isActive bool
	mu       sync.Mutex
}

// GetDatabase returns the cached database or fetches it using the db route and
// caches the result.
func (d *databaseInfo) GetDatabase(ctx context.Context, tc *client.TeleportClient) (types.Database, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.database != nil {
		return d.database.Copy(), nil
	}
	// holding mutex across the api call to avoid multiple redundant api calls.
	database, err := getDatabase(ctx, tc, d.ServiceName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	d.database = database
	return d.database.Copy(), nil
}

// getDatabaseForDBCmd is a callback for dbcmd connect option.
func (d *databaseInfo) getDatabaseForDBCmd(ctx context.Context, tc *client.TeleportClient, serviceName string) (types.Database, error) {
	if serviceName != d.ServiceName {
		return nil, trace.BadParameter("expect database service %s but got %s", d.ServiceName, serviceName)
	}
	return d.GetDatabase(ctx, tc)
}

// chooseOneDatabase is a helper func that returns either the only database in a
// list of databases or returns a database that matches the selector name
// or unambiguous discovered name exactly, otherwise an error.
func chooseOneDatabase(cf *CLIConf, databases types.Databases) (types.Database, error) {
	selectors := newDatabaseResourceSelectors(cf)
	// Check if the name matches any database exactly and, if so, choose
	// that database over any others.
	for _, db := range databases {
		if db.GetName() == selectors.name {
			logger.DebugContext(cf.Context, "Selected database by exact name match", "database", db.GetName())
			return db, nil
		}
	}
	// look for a single database with a matching discovered name label.
	if dbs := findDatabasesByDiscoveredName(databases, selectors.name); len(dbs) > 0 {
		names := make([]string, 0, len(dbs))
		for _, db := range dbs {
			names = append(names, db.GetName())
		}
		logger.DebugContext(cf.Context, "Choosing amongst databases by discovered name", "databases", names)
		databases = dbs
	}
	if len(databases) == 1 {
		logger.DebugContext(cf.Context, "Selected database", "database", databases[0].GetName())
		return databases[0], nil
	}

	// error - we need exactly one database.
	if len(databases) == 0 {
		return nil, trace.NotFound(
			"%v not found, use '%v' to see registered databases", selectors,
			formatDatabaseListCommand(cf.SiteName))
	}
	errMsg := formatAmbiguousDB(cf, selectors, databases)
	return nil, trace.BadParameter(errMsg)
}

// findDatabasesByDiscoveredName returns all databases that have a discovered
// name label that matches the given name.
func findDatabasesByDiscoveredName(databases types.Databases, name string) types.Databases {
	var out types.Databases
	for _, db := range databases {
		discoveredName, ok := db.GetLabel(types.DiscoveredNameLabel)
		if ok && discoveredName == name {
			out = append(out, db)
		}
	}
	return out
}

// getDatabase gets a database using its full name.
func getDatabase(ctx context.Context, tc *client.TeleportClient, name string) (types.Database, error) {
	matchName := makeNamePredicate(name)
	databases, err := listDatabasesWithPredicate(ctx, tc, matchName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(databases) == 0 {
		return nil, trace.NotFound("database %q not found among registered databases in cluster %v", name, tc.SiteName)
	}
	return databases[0], nil
}

func getDatabaseServers(ctx context.Context, tc *client.TeleportClient, name string) ([]types.DatabaseServer, error) {
	var databases []types.DatabaseServer

	err := client.RetryWithRelogin(ctx, tc, func() error {
		matchName := makeNamePredicate(name)

		var err error
		predicate := makePredicateConjunction(matchName, tc.PredicateExpression)
		logger.DebugContext(ctx, "Listing databases with predicate and labels", "predicate", predicate, "labels", tc.Labels)

		databases, err = tc.ListDatabaseServersWithFilters(ctx, &proto.ListResourcesRequest{
			Namespace:           apidefaults.Namespace,
			ResourceType:        types.KindDatabaseServer,
			PredicateExpression: predicate,
			Labels:              tc.Labels,
			UseSearchAsRoles:    tc.UseSearchAsRoles,
		})
		return trace.Wrap(err)
	})

	return databases, trace.Wrap(err)
}

// getDatabaseByNameOrDiscoveredName fetches a database that unambiguously
// matches a given name or a discovered name label.
func getDatabaseByNameOrDiscoveredName(cf *CLIConf, tc *client.TeleportClient, activeRoutes []tlsca.RouteToDatabase) (types.Database, error) {
	predicate := makeDiscoveredNameOrNamePredicate(cf.DatabaseService)
	databases, err := listDatabasesWithPredicate(cf.Context, tc, predicate)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if activeDBs := filterActiveDatabases(activeRoutes, databases); len(activeDBs) > 0 {
		names := make([]string, 0, len(activeDBs))
		for _, db := range activeDBs {
			names = append(names, db.GetName())
		}
		logger.DebugContext(cf.Context, "Choosing a database amongst active databases", "databases", names)
		// preferentially choose from active databases if any of them match.
		return chooseOneDatabase(cf, activeDBs)
	}
	return chooseOneDatabase(cf, databases)
}

func getDatabaseByNameOrDiscoveredNameUsingSearchAsRoles(cf *CLIConf, tc *client.TeleportClient) (types.Database, error) {
	tc.UseSearchAsRoles = true
	defer func() {
		tc.UseSearchAsRoles = false
	}()
	db, err := getDatabaseByNameOrDiscoveredName(cf, tc, nil)
	return db, trace.Wrap(err)
}

func filterActiveDatabases(routes []tlsca.RouteToDatabase, databases types.Databases) types.Databases {
	databasesByName := databases.ToMap()
	var out types.Databases
	for _, route := range routes {
		if db, ok := databasesByName[route.ServiceName]; ok {
			out = append(out, db)
		}
	}
	return out
}

// listDatabasesWithPredicate is a helper func for listing databases using
// a given additional predicate expression. If the teleport client already
// has a predicate expression, the predicates are combined with a logical AND.
func listDatabasesWithPredicate(ctx context.Context, tc *client.TeleportClient, predicate string) (types.Databases, error) {
	var databases []types.Database
	err := client.RetryWithRelogin(ctx, tc, func() error {
		var err error
		predicate := makePredicateConjunction(predicate, tc.PredicateExpression)
		logger.DebugContext(ctx, "Listing databases with predicate and labels", "predicate", predicate, "labels", tc.Labels)
		databases, err = tc.ListDatabases(ctx, &proto.ListResourcesRequest{
			Namespace:           apidefaults.Namespace,
			ResourceType:        types.KindDatabaseServer,
			PredicateExpression: predicate,
			Labels:              tc.Labels,
			UseSearchAsRoles:    tc.UseSearchAsRoles,
		})
		return trace.Wrap(err)
	})
	return databases, trace.Wrap(err)
}

func makeDiscoveredNameOrNamePredicate(name string) string {
	matchName := makeNamePredicate(name)
	matchDiscoveredName := makeDiscoveredNamePredicate(name)
	return makePredicateDisjunction(matchName, matchDiscoveredName)
}

func makeDiscoveredNamePredicate(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	return fmt.Sprintf(`labels[%q] == %q`, types.DiscoveredNameLabel, name)
}

func makeNamePredicate(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	return fmt.Sprintf(`name == %q`, name)
}

// makePredicateConjunction combines two predicate expressions into one
// expression as a conjunction (logical AND) of the expressions.
func makePredicateConjunction(a, b string) string {
	return combinePredicateExpressions(a, b, "&&")
}

// makePredicateDisjunction combines two predicate expressions into one
// expression as a disjunction (logical OR) of the expressions.
func makePredicateDisjunction(a, b string) string {
	return combinePredicateExpressions(a, b, "||")
}

// combinePredicateExpressions combines two predicate expressions into one
// expression with the given operator.
func combinePredicateExpressions(a, b, op string) string {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	switch {
	case a == "":
		return b
	case b == "":
		return a
	case a == b:
		return a
	default:
		return fmt.Sprintf("(%v) %v (%v)", a, op, b)
	}
}

// getDefaultDBUser enumerates the allowed database users for a given database
// and selects one if it is the only non-wildcard database user allowed.
// Returns an error if there are no allowed database users or more than one.
func getDefaultDBUser(db types.Database, checker services.AccessChecker) (string, error) {
	var extraUsers []string
	if db.GetProtocol() == defaults.ProtocolRedis {
		// Check for the Redis default username "default" in the same way as
		// Redis does. This way if the wildcard is the only allowed db_user,
		// we will select this as the default db user.
		// ref: https://redis.io/commands/auth
		extraUsers = append(extraUsers, defaults.DefaultRedisUsername)
	}
	// Note that EnumerateDatabaseUsers also calculates the username when
	// auto-user provisioning is enabled for this database.
	dbUsers, err := checker.EnumerateDatabaseUsers(db, extraUsers...)
	if err != nil {
		return "", trace.Wrap(err)
	}
	allowed := dbUsers.Allowed()
	if len(allowed) == 1 {
		return allowed[0], nil
	}
	// anything else is an error.
	if dbUsers.WildcardAllowed() {
		allowed = append([]string{types.Wildcard}, allowed...)
	}
	if len(allowed) == 0 {
		errMsg := "you are not allowed access to any database user for %v, " +
			"ask a cluster administrator to ensure your Teleport user has appropriate db_users set"
		return "", trace.AccessDenied(errMsg, db.GetName())
	}
	errMsg := fmt.Sprintf("please provide the database user using the --db-user flag, "+
		"allowed database users for %v: %v", db.GetName(), allowed)
	if dbUsers.WildcardAllowed() {
		denied := dbUsers.Denied()
		if len(denied) > 0 {
			errMsg += fmt.Sprintf(" except %v", denied)
		}
	}
	return "", trace.BadParameter(errMsg)
}

// isDatabaseUserRequired returns whether the --db-user flag is required for
// the db protocol.
func isDatabaseUserRequired(protocol string) bool {
	return role.RequireDatabaseUserMatcher(protocol)
}

// isDatabaseNameRequired returns whether the --db-name flag is required for
// the db protocol.
func isDatabaseNameRequired(protocol string) bool {
	if role.RequireDatabaseNameMatcher(protocol) {
		return true
	}
	switch protocol {
	case defaults.ProtocolOracle:
		// Always require database name for the Oracle protocol.
		return true
	}
	return false
}

// getDefaultDBName enumerates the allowed database names for a given database
// and selects one if it is the only non-wildcard database name allowed.
// Returns an error if there are no allowed database names or more than one.
func getDefaultDBName(db types.Database, checker services.AccessChecker) (string, error) {
	dbNames := checker.EnumerateDatabaseNames(db)
	allowed := dbNames.Allowed()
	if len(allowed) == 1 {
		return allowed[0], nil
	}
	// anything else is an error.
	if dbNames.WildcardAllowed() {
		allowed = append([]string{types.Wildcard}, allowed...)
	}
	if len(allowed) == 0 {
		errMsg := "you are not allowed access to any database name for %v, " +
			"ask a cluster administrator to ensure your Teleport user has appropriate db_names set"
		return "", trace.AccessDenied(errMsg, db.GetName())
	}
	errMsg := fmt.Sprintf("please provide the database name using the --db-name flag, "+
		"allowed database names for %v: %v", db.GetName(), allowed)
	if dbNames.WildcardAllowed() {
		denied := dbNames.Denied()
		if len(denied) > 0 {
			errMsg += fmt.Sprintf(" except %v", denied)
		}
	}
	return "", trace.BadParameter(errMsg)
}

func needDatabaseRelogin(cf *CLIConf, tc *client.TeleportClient, route tlsca.RouteToDatabase, profile *client.ProfileStatus, requires *dbLocalProxyRequirement) (bool, error) {
	if (requires.localProxy && requires.tunnel) || cf.LocalProxyTunnel {
		switch route.Protocol {
		case defaults.ProtocolOracle:
			// Oracle Protocol needs to generate a local configuration files.
			// thus even is tunnel mode was requested the login flow should check
			// if the Oracle client files should be updated.
		default:
			// We don't need to login if using a local proxy tunnel,
			// because a local proxy tunnel will handle db login itself.
			return false, nil

		}
	}
	found := false
	activeDatabases, err := profile.DatabasesForCluster(tc.SiteName)
	if err != nil {
		return false, trace.Wrap(err)
	}

	for _, v := range activeDatabases {
		if v.ServiceName == route.ServiceName {
			found = true
		}
	}
	// database not found in active list of databases.
	if !found {
		return true, nil
	}

	// For database protocols where database username is encoded in client certificate like Mongo
	// check if the command line dbUser matches the encoded username in database certificate.
	dbInfoChanged, err := dbInfoHasChanged(cf, profile.DatabaseCertPathForCluster(tc.SiteName, route.ServiceName))
	if err != nil {
		return false, trace.Wrap(err)
	}
	if dbInfoChanged {
		return true, nil
	}
	// Call API and check if a user needs to use MFA to connect to the database.
	mfaRequired, err := isMFADatabaseAccessRequired(cf.Context, tc, route)
	return mfaRequired, trace.Wrap(err)
}

// maybeDatabaseLogin checks if cert is still valid. If not valid, trigger db login logic.
// returns a true/false indicating whether database login was triggered.
func maybeDatabaseLogin(cf *CLIConf, tc *client.TeleportClient, profile *client.ProfileStatus, dbInfo *databaseInfo, requires *dbLocalProxyRequirement) error {
	reloginNeeded, err := needDatabaseRelogin(cf, tc, dbInfo.RouteToDatabase, profile, requires)
	if err != nil {
		return trace.Wrap(err)
	}

	if reloginNeeded {
		return trace.Wrap(databaseLogin(cf, tc, dbInfo))
	}
	return nil
}

// dbInfoHasChanged checks if cliConf.DatabaseUser or cliConf.DatabaseName info has changed in the user database certificate.
func dbInfoHasChanged(cf *CLIConf, certPath string) (bool, error) {
	dbRoles := requestedDatabaseRoles(cf)
	if cf.DatabaseUser == "" && cf.DatabaseName == "" && len(dbRoles) == 0 {
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
		logger.DebugContext(cf.Context, "Will reissue database certificate for user",
			"current_user", cf.DatabaseUser,
			"previous_user", identity.RouteToDatabase.Username,
		)
		return true, nil
	}
	if cf.DatabaseName != "" && cf.DatabaseName != identity.RouteToDatabase.Database {
		logger.DebugContext(cf.Context, "Will reissue database certificate for database name",
			"current_database", cf.DatabaseName,
			"previous_database", identity.RouteToDatabase.Database,
		)
		return true, nil
	}

	if !apiutils.ContainSameUniqueElements(dbRoles, identity.RouteToDatabase.Roles) {
		logger.DebugContext(cf.Context, "Will reissue database certificate for database roles",
			"current_roles", dbRoles,
			"previous_roles", identity.RouteToDatabase.Roles,
		)
		return true, nil
	}
	return false, nil
}

// isMFADatabaseAccessRequired calls the IsMFARequired endpoint in order to get from user roles if access to the database
// requires MFA.
func isMFADatabaseAccessRequired(ctx context.Context, tc *client.TeleportClient, database tlsca.RouteToDatabase) (bool, error) {
	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	defer clusterClient.Close()

	dbParam := proto.RouteToDatabase{
		ServiceName: database.ServiceName,
		Protocol:    database.Protocol,
		Username:    database.Username,
		Database:    database.Database,
	}
	mfaResp, err := clusterClient.AuthClient.IsMFARequired(ctx, &proto.IsMFARequiredRequest{
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
func pickActiveDatabase(cf *CLIConf, tc *client.TeleportClient, activeRoutes []tlsca.RouteToDatabase) (*tlsca.RouteToDatabase, error) {
	if route, err := maybePickActiveDatabase(cf, activeRoutes); err == nil && route != nil {
		return route, nil
	} else if err != nil {
		return nil, trace.Wrap(err)
	}
	// check if any active database can possibly match.
	selectors := newDatabaseResourceSelectors(cf)
	if routes := filterRoutesByPrefix(activeRoutes, selectors.name); len(routes) == 0 {
		// no match is possible.
		return nil, trace.NotFound(formatDBNotLoggedIn(cf.SiteName, selectors))
	}

	db, err := getDatabaseByNameOrDiscoveredName(cf, tc, activeRoutes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if route, ok := findActiveDatabase(db.GetName(), activeRoutes); ok {
		return &route, nil
	}
	return nil, trace.NotFound(formatDBNotLoggedIn(cf.SiteName, selectors))
}

// maybePickActiveDatabase tries to pick a database automatically when selectors
// are not given, or by an exact name match of an active database when neither
// labels nor query are given.
// The route returned may be nil, indicating an active route could not be
// picked.
func maybePickActiveDatabase(cf *CLIConf, activeRoutes []tlsca.RouteToDatabase) (*tlsca.RouteToDatabase, error) {
	selectors := newDatabaseResourceSelectors(cf)
	if selectors.query == "" && selectors.labels == "" {
		if selectors.name == "" {
			switch len(activeRoutes) {
			case 0:
				return nil, trace.NotFound(formatDBNotLoggedIn(cf.SiteName, selectors))
			case 1:
				logger.DebugContext(cf.Context, "Auto-selecting the only active database", "database", activeRoutes[0].ServiceName)
				return &activeRoutes[0], nil
			default:
				return nil, trace.BadParameter(formatChooseActiveDB(activeRoutes))
			}
		}
		if route, ok := findActiveDatabase(selectors.name, activeRoutes); ok {
			logger.DebugContext(cf.Context, "Selected active database by name", "database", route.ServiceName)
			return &route, nil
		}
	}
	return nil, nil
}

// getDatabasesForLogout selects databases for logout in "tsh db logout".
func getDatabasesForLogout(cf *CLIConf, tc *client.TeleportClient, activeRoutes []tlsca.RouteToDatabase) ([]tlsca.RouteToDatabase, error) {
	selectors := newDatabaseResourceSelectors(cf)
	if selectors.IsEmpty() {
		// if db name, labels, query was not given, logout of all databases.
		return activeRoutes, nil
	}
	route, err := pickActiveDatabase(cf, tc, activeRoutes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []tlsca.RouteToDatabase{*route}, nil
}

// findActiveDatabase returns a database route and a bool indicating whether
// the route was found.
func findActiveDatabase(name string, activeRoutes []tlsca.RouteToDatabase) (tlsca.RouteToDatabase, bool) {
	for _, r := range activeRoutes {
		if r.ServiceName == name {
			return r, true
		}
	}
	return tlsca.RouteToDatabase{}, false
}

func filterRoutesByPrefix(routes []tlsca.RouteToDatabase, prefix string) []tlsca.RouteToDatabase {
	var out []tlsca.RouteToDatabase
	for _, r := range routes {
		if strings.HasPrefix(r.ServiceName, prefix) {
			out = append(out, r)
		}
	}
	return out
}

func formatStaleDBCert(clusterFlag, name string) string {
	return fmt.Sprintf("you are logged into a database that no longer exists in the cluster (remove it with '%v %v')",
		formatDatabaseLogoutCommand(clusterFlag), name)
}

func formatChooseActiveDB(routes []tlsca.RouteToDatabase) string {
	var services []string
	for _, r := range routes {
		services = append(services, r.ServiceName)
	}
	return fmt.Sprintf("multiple databases are available (%v), please specify one by name, --labels, or --query",
		strings.Join(services, ", "))
}

func formatDBNotLoggedIn(clusterFlag string, selectors resourceSelectors) string {
	if selectors.IsEmpty() {
		return fmt.Sprintf(
			"please login using '%v' first (use '%v' to see registered databases)",
			formatDatabaseLoginCommand(clusterFlag),
			formatDatabaseListCommand(clusterFlag),
		)
	}
	return fmt.Sprintf("not logged into %s", selectors)
}

func formatDatabaseLogoutCommand(clusterFlag string) string {
	return formatTSHCommand("tsh db logout", clusterFlag)
}

func formatDatabaseLoginCommand(clusterFlag string) string {
	return formatTSHCommand("tsh db login", clusterFlag)
}

func formatDatabaseListCommand(clusterFlag string) string {
	return formatTSHCommand("tsh db ls", clusterFlag)
}

func formatTSHCommand(cmd, clusterFlag string) string {
	if clusterFlag == "" {
		return cmd
	}
	return fmt.Sprintf("%v --cluster=%v", cmd, clusterFlag)
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
	needUser := isDatabaseUserRequired(active.Protocol)
	needDatabase := isDatabaseNameRequired(active.Protocol)

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

// dbLocalProxyRequirement describes local proxy requirements for connecting to a database.
type dbLocalProxyRequirement struct {
	// localProxy is whether a local proxy is required to connect.
	localProxy bool
	// localProxyReasons is a list of reasons for why local proxy is required.
	localProxyReasons []string
	// tunnel is whether a local proxy tunnel is required to connect.
	tunnel bool
	// tunnelReasons is a list of reasons for why a tunnel is required.
	tunnelReasons []string
}

// addLocalProxy sets the local proxy requirement and appends reasons.
func (r *dbLocalProxyRequirement) addLocalProxy(reasons ...string) {
	r.localProxy = true
	r.localProxyReasons = append(r.localProxyReasons, reasons...)
}

// addLocalProxyWithTunnel sets the local proxy and tunnel requirements,
// and appends reasons for both.
func (r *dbLocalProxyRequirement) addLocalProxyWithTunnel(reasons ...string) {
	r.addLocalProxy(reasons...)
	r.tunnel = true
	r.tunnelReasons = append(r.tunnelReasons, reasons...)
}

// getDBLocalProxyRequirement determines what local proxy settings are required
// for a given database.
func getDBLocalProxyRequirement(tc *client.TeleportClient, route tlsca.RouteToDatabase) *dbLocalProxyRequirement {
	var out dbLocalProxyRequirement
	if tc.PrivateKeyPolicy.IsHardwareKeyPolicy() {
		out.addLocalProxyWithTunnel(formatKeyPolicyReason(tc.PrivateKeyPolicy))
	}

	// When Proxy is behind a load balancer and the database requires the web
	// port, a local proxy must be used so the TLS routing request can be
	// upgraded, regardless whether Proxy is in single or separate port mode.
	if tc.TLSRoutingConnUpgradeRequired && tc.DoesDatabaseUseWebProxyHostPort(route) {
		out.addLocalProxy("Teleport Proxy is behind a layer 7 load balancer or reverse proxy")
	}

	switch route.Protocol {
	case defaults.ProtocolSnowflake,
		defaults.ProtocolDynamoDB,
		defaults.ProtocolSQLServer,
		defaults.ProtocolCassandra,
		defaults.ProtocolOracle,
		defaults.ProtocolClickHouse,
		defaults.ProtocolSpanner:

		// Some protocols only work in the local tunnel mode.
		out.addLocalProxyWithTunnel(formatDBProtocolReason(route.Protocol))
	case defaults.ProtocolMySQL:
		// When TLS routing is enabled and MySQL is listening on the web port,
		// a local proxy is required to connect. With a separate port, MySQL
		// does not require a local proxy even if TLS routing is enabled.
		if tc.TLSRoutingEnabled && tc.DoesDatabaseUseWebProxyHostPort(route) {
			out.addLocalProxy(fmt.Sprintf("%v and %v",
				formatDBProtocolReason(route.Protocol),
				formatTLSRoutingReason(tc.SiteName)))
		}
	}
	return &out
}

func getDBConnectLocalProxyRequirement(ctx context.Context, tc *client.TeleportClient, route tlsca.RouteToDatabase, tunnelFlag bool) *dbLocalProxyRequirement {
	r := getDBLocalProxyRequirement(tc, route)
	// Forces local proxy tunnel when --tunnel is on.
	if !r.tunnel && tunnelFlag {
		r.addLocalProxyWithTunnel(dbConnectRequireReasonTunnelFlag)
	}
	// Forces local proxy when cluster has TLS routing enabled.
	if !r.localProxy && tc.TLSRoutingEnabled {
		r.addLocalProxy(formatTLSRoutingReason(tc.SiteName))
	}
	switch route.Protocol {
	case defaults.ProtocolElasticsearch, defaults.ProtocolOpenSearch:
		// ElasticSearch and OpenSearch access can work without a local proxy tunnel,
		// but not via `tsh db connect`.
		// (elasticsearch-sql-cli and opensearchsql cannot be configured to use specific certs).
		r.addLocalProxyWithTunnel(formatDBProtocolReason(route.Protocol))
	}
	if r.localProxy && r.tunnel {
		// don't check if MFA is required, because a local proxy tunnel is
		// already required. this avoids an extra API call.
		return r
	}
	// Call API and check if a user needs to use MFA to connect to the database.
	mfaRequired, err := isMFADatabaseAccessRequired(ctx, tc, route)
	if err != nil {
		logger.DebugContext(ctx, "error getting MFA requirement for database",
			"database", route.ServiceName,
			"error", err,
		)
	} else if mfaRequired {
		// When MFA is required, we should require a local proxy tunnel,
		// because the local proxy tunnel can hold database MFA certs in-memory
		// without a restricted 1-minute TTL. This is better for user experience.
		r.addLocalProxyWithTunnel("MFA is required to connect to the database")
	}
	return r
}

// formatKeyPolicyReason is a helper func that formats a private key policy "reason".
// The "reason" is used to explain why something happened.
func formatKeyPolicyReason(policy keys.PrivateKeyPolicy) string {
	return fmt.Sprintf("private key policy is %v", policy)
}

// formatDBProtocolReason is a helper func that formats a database protocol
// "reason".
// The "reason" is used to explain why something happened.
func formatDBProtocolReason(protocol string) string {
	return fmt.Sprintf("database protocol is %v",
		defaults.ReadableDatabaseProtocol(protocol))
}

// formatTLSRoutingReason is a helper func that formats a cluster proxy
// TLS routing enabled "reason".
// The "reason" is used to explain why something happened.
func formatTLSRoutingReason(siteName string) string {
	return fmt.Sprintf("cluster %v proxy is using TLS routing",
		siteName)
}

// formatDbCmdUnsupported is a helper func that formats a generic unsupported DB error message.
// The "reasons" arguments, if given, should specify condition for which this DB subcommand
// is not supported, e.g. "TLS routing is enabled" or "using a local proxy without the --tunnel flag".
func formatDbCmdUnsupported(cf *CLIConf, route tlsca.RouteToDatabase, reasons ...string) string {
	templateData := map[string]any{
		"command":      cf.CommandWithBinary(),
		"alternatives": getDbCmdAlternatives(cf.SiteName, route),
		"reasons":      reasons,
	}

	buf := bytes.NewBuffer(nil)
	_ = dbCmdUnsupportedTemplate.Execute(buf, templateData)
	return buf.String()
}

// formatDbCmdUnsupportedDBProtocol is a helper func that formats an unsupported DB protocol error message.
func formatDbCmdUnsupportedDBProtocol(cf *CLIConf, route tlsca.RouteToDatabase) string {
	reason := formatDBProtocolReason(route.Protocol)
	return formatDbCmdUnsupported(cf, route, reason)
}

// getDbCmdAlternatives is a helper func that returns alternative tsh commands for connecting to a database.
func getDbCmdAlternatives(clusterFlag string, route tlsca.RouteToDatabase) []string {
	var alts []string
	if protocolSupportsInteractiveMode(route.Protocol) {
		// prefer displaying the connect command as the first suggested command alternative.
		alts = append(alts, formatDatabaseConnectCommand(clusterFlag, route))
	}
	// all db protocols support this command.
	alts = append(alts, formatDatabaseProxyCommand(clusterFlag, route))
	return alts
}

// formatAmbiguousDB is a helper func that formats an ambiguous database error
// message.
func formatAmbiguousDB(cf *CLIConf, selectors resourceSelectors, matchedDBs types.Databases) string {
	var activeDBs []tlsca.RouteToDatabase
	if profile, err := cf.ProfileStatus(); err == nil {
		if dbs, err := profile.DatabasesForCluster(cf.SiteName); err == nil {
			activeDBs = dbs
		}
	}
	// Pass a nil access checker to avoid making a proxy roundtrip.
	// Access info isn't relevant to an ambiguity error anyway.
	var checker services.AccessChecker
	var sb strings.Builder
	verbose := true
	showDatabasesAsText(cf, &sb, matchedDBs, activeDBs, checker, verbose)

	listCommand := formatDatabaseListCommand(cf.SiteName)
	fullNameExample := matchedDBs[0].GetName()
	return formatAmbiguityErrTemplate(cf, selectors, listCommand, sb.String(), fullNameExample)
}

// resourceSelectors is a helper struct for gathering up the selectors for a
// resource, as an aggregate of name, labels, and predicate query.
type resourceSelectors struct {
	kind,
	name,
	labels,
	query string
}

// String returns the resource selectors as a formatted string.
// Example:
// command: `tsh db connect foo --labels k1=v1 --query 'labels["k2"]=="v2"'`
// output: database "foo" with labels "k1=v1" with query (labels["k2"]=="v2")
func (r resourceSelectors) String() string {
	out := r.kind
	if r.name != "" {
		out = fmt.Sprintf("%s %q", out, r.name)
	}
	if len(r.labels) > 0 {
		out = fmt.Sprintf("%s with labels %q", out, r.labels)
	}
	if len(r.query) > 0 {
		out = fmt.Sprintf("%s with query (%s)", out, r.query)
	}
	return strings.TrimSpace(out)
}

// IsEmpty returns whether the selectors (except kind) are empty.
func (r resourceSelectors) IsEmpty() bool {
	return r.name == "" && r.labels == "" && r.query == ""
}

func newDatabaseResourceSelectors(cf *CLIConf) resourceSelectors {
	return resourceSelectors{
		kind:   "database",
		name:   cf.DatabaseService,
		labels: cf.Labels,
		query:  cf.PredicateExpression,
	}
}

// formatAmbiguityErrTemplate is a helper func that formats an ambiguous
// resource error message.
func formatAmbiguityErrTemplate(cf *CLIConf, selectors resourceSelectors, listCommand, matchTable, fullNameExample string) string {
	data := map[string]any{
		"command":     cf.CommandWithBinary(),
		"listCommand": strings.TrimSpace(listCommand),
		"kind":        strings.TrimSpace(selectors.kind),
		"matchTable":  strings.TrimSpace(matchTable),
		"example":     strings.TrimSpace(fullNameExample),
	}
	if !selectors.IsEmpty() {
		data["selectors"] = strings.TrimSpace(selectors.String())
	}
	var sb strings.Builder
	_ = ambiguityErrTemplate.Execute(&sb, data)
	return sb.String()
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
	// dbConnectRequireReasonTunnelFlag is the reason used in local proxy
	// requirement calculation when --tunnel flag is specified.
	dbConnectRequireReasonTunnelFlag = "--tunnel flag is specified"
)

var (
	// dbCmdUnsupportedTemplate is the error message printed when some
	// database subcommands are not supported.
	dbCmdUnsupportedTemplate = template.Must(template.New("").Parse(`"{{.command}}" is not supported{{if .reasons}} when:
{{- range $reason := .reasons }}
  - {{ $reason }}.
{{- end}}
{{- else}}.
{{- end}}
{{if eq (len .alternatives) 1}}
Please use the following command to connect to the database:
    {{index .alternatives 0 -}}{{else}}
Please use one of the following commands to connect to the database:
	{{- range .alternatives}}
    {{.}}{{end -}}
{{- end}}`))

	// dbConnectTemplate is the message printed after a successful "tsh db login" on how to connect.
	dbConnectTemplate = template.Must(template.New("").Parse(`Connection information for database "{{ .name }}" has been saved.

{{if .connectCommand -}}

You can now connect to it using the following command:

  {{.connectCommand}}

{{end -}}
{{if .configCommand -}}

You can view the connect command for the native database CLI client:

  {{ .configCommand }}

{{end -}}
{{if .proxyCommand -}}

You can start a local proxy for database GUI clients:

  {{ .proxyCommand }}

{{end -}}
`))

	// ambiguityErrTemplate is the error message printed when a resource is
	// specified ambiguously by name prefix and/or labels.
	ambiguityErrTemplate = template.Must(template.New("").Parse(`{{if .selectors -}}
{{ .selectors }} matches multiple {{ .kind }}s:
{{- else -}}
multiple {{ .kind }}s are available:
{{- end }}

{{ .matchTable }}

Hint: use '{{ .listCommand }} -v' or '{{ .listCommand }} --format=[json|yaml]' to list all {{ .kind }}s with full details.
Hint: try selecting the {{ .kind }} with a more specific name (ex: {{ .command }} {{ .example }}).
Hint: try selecting the {{ .kind }} with additional --labels or --query predicate.
`))
)
