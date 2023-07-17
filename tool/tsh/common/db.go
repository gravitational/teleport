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

package common

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"text/template"

	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/exp/slices"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
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

	var proxy *client.ProxyClient
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		proxy, err = tc.ConnectToProxy(cf.Context)
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxy.Close()

	databases, err := proxy.FindDatabasesByFiltersForCluster(cf.Context, *tc.ResourceFilter(types.KindDatabaseServer), tc.SiteName)
	if err != nil {
		return trace.Wrap(err)
	}

	accessChecker, err := accessCheckerForRemoteCluster(cf.Context, profile, proxy, tc.SiteName)
	if err != nil {
		log.Debugf("Failed to fetch user roles: %v.", err)
	}

	activeDatabases, err := profile.DatabasesForCluster(tc.SiteName)
	if err != nil {
		return trace.Wrap(err)
	}

	sort.Sort(types.Databases(databases))
	return trace.Wrap(showDatabases(cf.Stdout(), cf.SiteName, databases, activeDatabases, accessChecker, cf.Format, cf.Verbose))
}

func accessCheckerForRemoteCluster(ctx context.Context, profile *client.ProfileStatus, proxy *client.ProxyClient, clusterName string) (services.AccessChecker, error) {
	cluster, err := proxy.ConnectToCluster(ctx, clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer cluster.Close()

	accessChecker, err := services.NewAccessCheckerForRemoteCluster(ctx, profile.AccessInfo(), clusterName, cluster)
	return accessChecker, trace.Wrap(err)
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

			logger := log.WithField("cluster", cluster.name)
			databases, err := cluster.proxy.FindDatabasesByFiltersForCluster(ctx, cluster.req, cluster.name)
			if err != nil {
				logger.Errorf("Failed to get databases: %v.", err)

				mu.Lock()
				errors = append(errors, trace.ConnectionProblem(err, "failed to list databases for cluster %s: %v", cluster.name, err))
				mu.Unlock()
				return nil
			}

			accessChecker, err := accessCheckerForRemoteCluster(ctx, cluster.profile, cluster.proxy, cluster.name)
			if err != nil {
				log.Debugf("Failed to fetch user roles: %v.", err)
			}

			localDBListings := make(databaseListings, 0, len(databases))
			for _, database := range databases {
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

	return trace.NewAggregate(errors...)
}

// onDatabaseLogin implements "tsh db login" command.
func onDatabaseLogin(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	dbInfo, err := newDatabaseInfo(cf, tc, nil)
	if err != nil {
		return trace.Wrap(err)
	}

	database, err := dbInfo.GetDatabase(cf, tc)
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

	// DynamoDB does not support a connect command, so don't try to print one.
	if database.GetProtocol() != defaults.ProtocolDynamoDB {
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

func databaseLogin(cf *CLIConf, tc *client.TeleportClient, dbInfo *databaseInfo) error {
	log.Debugf("Fetching database access certificate for %s on cluster %v.", dbInfo.RouteToDatabase, tc.SiteName)

	profile, err := tc.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}

	var key *client.Key
	// Identity files themselves act as the database credentials (if any), so
	// don't bother fetching new certs.
	if profile.IsVirtual {
		log.Info("Note: already logged in due to an identity file (`-i ...`); will only update database config files.")
	} else {
		if err = client.RetryWithRelogin(cf.Context, tc, func() error {
			key, err = tc.IssueUserCertsWithMFA(cf.Context, client.ReissueParams{
				RouteToCluster: tc.SiteName,
				RouteToDatabase: proto.RouteToDatabase{
					ServiceName: dbInfo.ServiceName,
					Protocol:    dbInfo.Protocol,
					Username:    dbInfo.Username,
					Database:    dbInfo.Database,
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

	if dbInfo.Protocol == defaults.ProtocolOracle {
		if err := generateDBLocalProxyCert(key, profile); err != nil {
			return trace.Wrap(err)
		}
		err = oracle.GenerateClientConfiguration(key, dbInfo.RouteToDatabase, profile)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// Refresh the profile.
	profile, err = tc.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
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
	logout, _, err := filterActiveDatabases(cf.Context, tc, activeRoutes)
	if err != nil {
		return trace.Wrap(err)
	}

	if profile.IsVirtual {
		log.Info("Note: an identity file is in use (`-i ...`); will only update database config files.")
	}

	for _, db := range logout {
		if err := databaseLogout(tc, db, profile.IsVirtual); err != nil {
			return trace.Wrap(err)
		}
	}
	msg, err := makeLogoutMessage(cf, logout, activeRoutes)
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
	case 0:
		selectors := resourceSelectors{
			kind:   "database",
			name:   cf.DatabaseService,
			labels: cf.Labels,
			query:  cf.PredicateExpression,
		}
		return "", trace.NotFound("Not logged into %v", selectors)
	case 1:
		return fmt.Sprintf("Logged out of database %v", logout[0].ServiceName), nil
	case len(activeRoutes):
		return "Logged out of all databases", nil
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

	database, err := pickActiveDatabase(cf, tc)
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
	database, err := pickActiveDatabase(cf, tc)
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
func maybeStartLocalProxy(ctx context.Context, cf *CLIConf,
	tc *client.TeleportClient, profile *client.ProfileStatus,
	dbInfo *databaseInfo, rootClusterName string,
	requires *dbLocalProxyRequirement,
) ([]dbcmd.ConnectCommandFunc, error) {
	if !requires.localProxy {
		return nil, nil
	}
	if requires.tunnel {
		log.Debugf("Starting local proxy tunnel because: %v", strings.Join(requires.tunnelReasons, ", "))
	} else {
		log.Debugf("Starting local proxy because: %v", strings.Join(requires.localProxyReasons, ", "))
	}

	listener, err := createLocalProxyListener("localhost:0", dbInfo.RouteToDatabase, profile)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	opts, err := prepareLocalProxyOptions(&localProxyConfig{
		cf:               cf,
		tc:               tc,
		profile:          profile,
		dbInfo:           dbInfo,
		autoReissueCerts: requires.tunnel,
		tunnel:           requires.tunnel,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	lp, err := alpnproxy.NewLocalProxy(makeBasicLocalProxyConfig(cf, tc, listener), opts...)
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
	// autoReissueCerts indicates whether a cert auto reissuer should be used
	// for the local proxy to keep certificates valid.
	// - when `tsh db connect` needs to tunnel it will set this field.
	// - when `tsh proxy db` is used with `--tunnel` cli flag it will set this field.
	autoReissueCerts bool
	// tunnel controls whether client certs will always be used to dial upstream.
	tunnel bool
}

func createLocalProxyListener(addr string, route tlsca.RouteToDatabase, profile *client.ProfileStatus) (net.Listener, error) {
	if route.Protocol == defaults.ProtocolOracle {
		localCert, err := tls.LoadX509KeyPair(
			profile.DatabaseLocalCAPath(),
			profile.KeyPath(),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		l, err := tls.Listen("tcp", addr, &tls.Config{
			Certificates: []tls.Certificate{localCert},
			ServerName:   "localhost",
		})
		return l, trace.Wrap(err)
	}
	l, err := net.Listen("tcp", addr)
	return l, trace.Wrap(err)
}

// prepareLocalProxyOptions created localProxyOpts needed to create local proxy from localProxyConfig.
func prepareLocalProxyOptions(arg *localProxyConfig) ([]alpnproxy.LocalProxyConfigOpt, error) {
	opts := []alpnproxy.LocalProxyConfigOpt{
		alpnproxy.WithDatabaseProtocol(arg.dbInfo.Protocol),
		alpnproxy.WithClusterCAsIfConnUpgrade(arg.cf.Context, arg.tc.RootClusterCACertPool),
	}

	if !arg.tunnel && arg.dbInfo.Protocol == defaults.ProtocolPostgres {
		opts = append(opts, alpnproxy.WithCheckCertsNeeded())
	}

	// load certs if local proxy needs to be able to tunnel.
	// certs are needed for non-tunnel postgres cancel requests.
	if arg.tunnel || arg.dbInfo.Protocol == defaults.ProtocolPostgres {
		certs, err := getDBLocalProxyCerts(arg)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		opts = append(opts, alpnproxy.WithClientCerts(certs...))
	}

	if arg.autoReissueCerts {
		opts = append(opts, alpnproxy.WithMiddleware(client.NewDBCertChecker(arg.tc, arg.dbInfo.RouteToDatabase, nil)))
	}

	// To set correct MySQL server version DB proxy needs additional protocol.
	if !arg.tunnel && arg.dbInfo.Protocol == defaults.ProtocolMySQL {
		db, err := arg.dbInfo.GetDatabase(arg.cf, arg.tc)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		opts = append(opts, alpnproxy.WithMySQLVersionProto(db))
	}
	return opts, nil
}

// getDBLocalProxyCerts gets cert/key file specified by cli config, or
// if both are not specified then it tries to load certs from the profile.
// This is a helper func for preparing local proxy options.
func getDBLocalProxyCerts(arg *localProxyConfig) ([]tls.Certificate, error) {
	if arg.cf.LocalProxyCertFile != "" || arg.cf.LocalProxyKeyFile != "" {
		return getUserSpecifiedLocalProxyCerts(arg)
	}
	// if neither --cert-file nor --key-file are specified, load db cert from client store.
	cert, err := loadDBCertificate(arg.tc, arg.dbInfo.ServiceName)
	if err != nil {
		if arg.autoReissueCerts {
			// If using a reissuer, just return nil certs and let the reissuer
			// fetch new certs when the local proxy starts instead.
			// We don't do this for user specified certs (above), because it is
			// surprising UX to get a login prompt when a user passes
			// --cert-file/--key-file, and we don't know how the user wants to
			// proceed.
			return nil, nil
		}
		return nil, trace.Wrap(err)
	}
	return []tls.Certificate{cert}, nil
}

// getUserSpecifiedLocalProxyCerts loads certs from files specified by cli arguments.
// This is a helper func for preparing local proxy options.
func getUserSpecifiedLocalProxyCerts(arg *localProxyConfig) ([]tls.Certificate, error) {
	if arg.cf.LocalProxyCertFile == "" || arg.cf.LocalProxyKeyFile == "" {
		return nil, trace.BadParameter("both --cert-file and --key-file are required")
	}
	cert, err := keys.LoadX509KeyPair(arg.cf.LocalProxyCertFile, arg.cf.LocalProxyKeyFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []tls.Certificate{cert}, nil
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
	dbInfo, err := getDatabaseInfo(cf, tc)
	if err != nil {
		return trace.Wrap(err)
	}
	if dbInfo.Protocol == defaults.ProtocolDynamoDB {
		return trace.BadParameter(formatDbCmdUnsupportedDBProtocol(cf, dbInfo.RouteToDatabase))
	}

	requires := getDBConnectLocalProxyRequirement(cf.Context, tc, dbInfo.RouteToDatabase)
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
	opts = append(opts, dbcmd.WithLogger(log))

	if opts, err = maybeAddDBUserPassword(cf, tc, dbInfo, opts); err != nil {
		return trace.Wrap(err)
	}

	bb := dbcmd.NewCmdBuilder(tc, profile, dbInfo.RouteToDatabase, rootClusterName, opts...)
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

// getDatabaseInfo fetches information about the database from tsh profile if DB
// is active in profile and no labels or predicate query are given.
// Otherwise, the ListDatabases endpoint is called.
func getDatabaseInfo(cf *CLIConf, tc *client.TeleportClient) (*databaseInfo, error) {
	haveSelectors := len(tc.Labels) > 0 || tc.PredicateExpression != ""
	if !haveSelectors {
		// if selectors are given, we might incur an extra ListDatabases API
		// call here to match against an active database.
		// So try to pick an active database only when we don't have
		// selectors.
		if route, err := pickActiveDatabase(cf, tc); err == nil {
			return newDatabaseInfo(cf, tc, route)
		} else if err != nil && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
	}
	return newDatabaseInfo(cf, tc, nil)
}

// newDatabaseInfo makes a new databaseInfo from the given route to the db.
// It checks the route and sets defaults as needed for protocol, db user, or db
// name. If the route is not given or the remote database is needed for setting
// a default, the database is retrieved by calling ListDatabases API and cached.
func newDatabaseInfo(cf *CLIConf, tc *client.TeleportClient, route *tlsca.RouteToDatabase) (*databaseInfo, error) {
	dbInfo := &databaseInfo{}
	if route != nil {
		dbInfo.RouteToDatabase = *route
		// the only way we're going to have all this info populated is from an
		// active cert.
		if dbInfo.ServiceName != "" && dbInfo.Protocol != "" &&
			dbInfo.Username != "" && dbInfo.Database != "" {
			return dbInfo, nil
		}
	}
	db, err := dbInfo.GetDatabase(cf, tc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// now ensure the route name and protocol matches the db we fetched.
	dbInfo.ServiceName = db.GetName()
	dbInfo.Protocol = db.GetProtocol()
	return dbInfo, dbInfo.checkAndSetPrincipalDefaults(cf, tc, db)
}

// checkAndSetPrincipalDefaults checks the db route (schema) name and username,
// and sets them to defaults if necessary.
func (d *databaseInfo) checkAndSetPrincipalDefaults(cf *CLIConf, tc *client.TeleportClient, db types.Database) error {
	if cf.DatabaseUser != "" {
		d.Username = cf.DatabaseUser
	}
	if cf.DatabaseName != "" {
		d.Database = cf.DatabaseName
	}
	// If database has admin user defined, we're most likely using automatic
	// user provisioning so default to Teleport username unless database
	// username was provided explicitly.
	if d.Username == "" && db.GetAdminUser() != "" {
		log.Debugf("Defaulting to Teleport username %q as database username.", tc.Username)
		d.Username = tc.Username
	}
	// recheck to see if we can avoid fetching the roleset to set defaults.
	needDBUser := d.Username == "" && role.RequireDatabaseUserMatcher(d.Protocol)
	needDBName := d.Database == "" && role.RequireDatabaseNameMatcher(d.Protocol)
	if !needDBUser && !needDBName {
		return nil
	}

	profile, err := tc.ProfileStatus()
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

	checker, err := accessCheckerForRemoteCluster(cf.Context, profile, proxy, tc.SiteName)
	if err != nil {
		return trace.Wrap(err)
	}

	if needDBUser {
		dbUser, err := getDefaultDBUser(db, checker)
		if err != nil {
			return trace.Wrap(err)
		}
		log.Debugf("Defaulting to the allowed database user %q\n", dbUser)
		d.Username = dbUser
	}
	if needDBName {
		dbName, err := getDefaultDBName(db, checker)
		if err != nil {
			return trace.Wrap(err)
		}
		log.Debugf("Defaulting to the allowed database name %q\n", dbName)
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
	mu       sync.Mutex
}

// GetDatabase returns the cached database or fetches it using the db route and
// caches the result.
func (d *databaseInfo) GetDatabase(cf *CLIConf, tc *client.TeleportClient) (types.Database, error) {
	if d.ServiceName == "" && cf.DatabaseService == "" &&
		len(tc.Labels) == 0 && tc.PredicateExpression == "" {
		return nil, trace.BadParameter("specify a database service by name, --labels, or --query")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.database != nil {
		return d.database, nil
	}
	// holding mutex across the api call to avoid multiple redundant api calls.
	var databases types.Databases
	var err error
	name := d.ServiceName
	if name != "" {
		databases, err = listDatabasesByName(cf.Context, tc, name)
	} else {
		name = cf.DatabaseService
		// search by prefix if the db name comes from cli flag instead of cert.
		databases, err = listDatabasesByPrefix(cf.Context, tc, name)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(databases) != 1 {
		// error - we need exactly one database.
		selectors := resourceSelectors{
			kind:   "database",
			name:   name,
			labels: cf.Labels,
			query:  cf.PredicateExpression,
		}
		if len(databases) == 0 {
			return nil, trace.NotFound(
				"%v not found, use '%v' to see registered databases", selectors,
				formatDatabaseListCommand(cf.SiteName))
		}
		errMsg := formatAmbiguousDB(cf, selectors, databases)
		return nil, trace.BadParameter(errMsg)
	}

	d.database = databases[0]
	return d.database, nil
}

// listActiveDatabases lists databases that match active (logged in) databases.
func listActiveDatabases(ctx context.Context, tc *client.TeleportClient, routes []tlsca.RouteToDatabase) (types.Databases, error) {
	names := make([]string, 0, len(routes))
	for _, r := range routes {
		names = append(names, fmt.Sprintf("(name == %q)", r.ServiceName))
	}
	predicate := strings.Join(names, "||")
	return listDatabasesWithPredicate(ctx, tc, predicate)
}

// listDatabasesByName lists database that match a given name.
func listDatabasesByName(ctx context.Context, tc *client.TeleportClient, name string) (types.Databases, error) {
	predicate := fmt.Sprintf("name == %s", name)
	return listDatabasesWithPredicate(ctx, tc, predicate)
}

// listDatabasesByPrefix lists databases that match a given name prefix.
func listDatabasesByPrefix(ctx context.Context, tc *client.TeleportClient, prefix string) (types.Databases, error) {
	predicate := fmt.Sprintf(`hasPrefix(name, "%s")`, prefix)
	databases, err := listDatabasesWithPredicate(ctx, tc, predicate)
	if err == nil || !utils.IsPredicateError(err) {
		return databases, trace.Wrap(err)
	}
	// predicate error from using hasPrefix expression.
	// fallback to listing without the hasPrefix predicate and filtering
	// on client side for backwards compatibility.
	databases, err = listDatabasesWithPredicate(ctx, tc, "")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out types.Databases
	for _, db := range databases {
		if strings.HasPrefix(db.GetName(), prefix) {
			out = append(out, db)
		}
	}
	return out, nil
}

// listDatabasesWithPredicate is a helper func for listing databases using
// a given additional predicate expression. If the teleport client already
// has a predicate expression, the predicates are combined with a logical AND.
func listDatabasesWithPredicate(ctx context.Context, tc *client.TeleportClient, predicate string) (types.Databases, error) {
	if predicate == "" {
		predicate = tc.PredicateExpression
	} else if tc.PredicateExpression != "" {
		predicate = fmt.Sprintf("(%v) && (%v)", predicate, tc.PredicateExpression)
	}
	var databases []types.Database
	err := client.RetryWithRelogin(ctx, tc, func() error {
		var err error
		databases, err = tc.ListDatabases(ctx, &proto.ListResourcesRequest{
			Namespace:           tc.Namespace,
			ResourceType:        types.KindDatabaseServer,
			PredicateExpression: predicate,
			Labels:              tc.Labels,
		})
		return trace.Wrap(err)
	})
	return databases, trace.Wrap(err)
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
	dbUsers := checker.EnumerateDatabaseUsers(db, extraUsers...)
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
	if (requires.localProxy && requires.tunnel) || isLocalProxyTunnelRequested(cf) {
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
func isMFADatabaseAccessRequired(ctx context.Context, tc *client.TeleportClient, database tlsca.RouteToDatabase) (bool, error) {
	proxy, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	cluster, err := proxy.ConnectToCluster(ctx, tc.SiteName)
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
	mfaResp, err := cluster.IsMFARequired(ctx, &proto.IsMFARequiredRequest{
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
func pickActiveDatabase(cf *CLIConf, tc *client.TeleportClient) (*tlsca.RouteToDatabase, error) {
	profile, err := tc.ProfileStatus()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	routes, err := profile.DatabasesForCluster(tc.SiteName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(routes) == 0 {
		return nil, trace.NotFound("please login using 'tsh db login' first")
	}

	routes, databases, err := filterActiveDatabases(cf.Context, tc, routes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(routes) != 1 {
		// error - we need exactly one route.
		selectors := resourceSelectors{
			kind:   "database",
			name:   cf.DatabaseService,
			labels: cf.Labels,
			query:  cf.PredicateExpression,
		}
		if len(routes) == 0 {
			return nil, trace.NotFound("not logged into %v", selectors)
		}
		if len(databases) == 0 {
			// if not already given, try to fetch them so we can print full
			// the full `tsh db ls -v` table of ambiguously matching active DBs.
			databases, err = listActiveDatabases(cf.Context, tc, routes)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		errMsg := formatAmbiguousDB(cf, selectors, databases)
		return nil, trace.BadParameter(errMsg)
	}

	route := &routes[0]
	// If database user or name were provided on the CLI,
	// override the default ones.
	if cf.DatabaseUser != "" {
		route.Username = cf.DatabaseUser
	}
	if cf.DatabaseName != "" {
		route.Database = cf.DatabaseName
	}
	return route, nil
}

// filterActiveDatabases takes a list of active database routes and returns a
// filtered list and, possibly, their corresponding types.Databases.
// Callers should therefore not assume that the types.Databases are populated.
// Filtering is done by matching on database name, label, and query predicate
// selectors from the Teleport client.
// When only database name is given, filtering is done by name prefix, unless
// an active database name matches exactly, in which case all other active
// databases are filtered out - this is to avoid requiring additional selectors
// when a user gives an exact database name.
func filterActiveDatabases(ctx context.Context, tc *client.TeleportClient, activeRoutes []tlsca.RouteToDatabase) ([]tlsca.RouteToDatabase, types.Databases, error) {
	prefix := tc.DatabaseService
	if prefix == "" && len(activeRoutes) == 1 {
		prefix = activeRoutes[0].ServiceName
	}

	haveSelectors := len(tc.Labels) > 0 || tc.PredicateExpression != ""
	var selectedRoutes []tlsca.RouteToDatabase
	for _, db := range activeRoutes {
		if db.ServiceName == prefix && !haveSelectors {
			// short-circuit to select the exact match when we don't have
			// label or predicate selectors.
			return []tlsca.RouteToDatabase{db}, nil, nil
		}
		if strings.HasPrefix(db.ServiceName, prefix) {
			selectedRoutes = append(selectedRoutes, db)
		}
	}
	if len(selectedRoutes) == 0 || !haveSelectors {
		// nothing to filter further, avoid making API call.
		return selectedRoutes, nil, nil
	}

	// make a ListDatabases API call and match on full database name.
	databases, err := listDatabasesByPrefix(ctx, tc, prefix)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	selectedRoutes = nil
	var activeDBs types.Databases
	for _, route := range activeRoutes {
		for _, db := range databases {
			if db.GetName() == route.ServiceName {
				selectedRoutes = append(selectedRoutes, route)
				activeDBs = append(activeDBs, db)
			}
		}
	}
	return selectedRoutes, activeDBs, nil
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
	needUser := role.RequireDatabaseUserMatcher(active.Protocol)
	needDatabase := role.RequireDatabaseNameMatcher(active.Protocol)

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
	switch tc.PrivateKeyPolicy {
	case keys.PrivateKeyPolicyHardwareKey, keys.PrivateKeyPolicyHardwareKeyTouch:
		out.addLocalProxyWithTunnel(formatKeyPolicyReason(tc.PrivateKeyPolicy))
	}

	// When Proxy is behind a load balancer and the database requires the web
	// port, a local proxy must be used so the TLS routing request can be
	// upgraded, regardless whether Proxy is in single or separate port mode.
	if tc.TLSRoutingConnUpgradeRequired && tc.DoesDatabaseUseWebProxyHostPort(route) {
		out.addLocalProxy("Teleport Proxy is behind a load balancer")
	}

	switch route.Protocol {
	case defaults.ProtocolSnowflake,
		defaults.ProtocolDynamoDB,
		defaults.ProtocolSQLServer,
		defaults.ProtocolCassandra,
		defaults.ProtocolOracle:

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

func getDBConnectLocalProxyRequirement(ctx context.Context, tc *client.TeleportClient, route tlsca.RouteToDatabase) *dbLocalProxyRequirement {
	r := getDBLocalProxyRequirement(tc, route)
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
		log.WithError(err).Debugf("error getting MFA requirement for database %v",
			route.ServiceName)
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
	switch route.Protocol {
	case defaults.ProtocolDynamoDB:
		// DynamoDB only works with a local proxy tunnel and there is no "shell-like" cli, so `tsh db connect` doesn't make sense.
	default:
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
	showDatabasesAsText(&sb, cf.SiteName, matchedDBs, activeDBs, checker, verbose)

	listCommand := formatDatabaseListCommand(cf.SiteName)
	return formatAmbiguityErrTemplate(cf, selectors, listCommand, sb.String())
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

// formatAmbiguityErrTemplate is a helper func that formats an ambiguous
// resource error message.
func formatAmbiguityErrTemplate(cf *CLIConf, selectors resourceSelectors, listCommand, matchTable string) string {
	data := map[string]any{
		"command":     cf.CommandWithBinary(),
		"selectors":   strings.TrimSpace(selectors.String()),
		"listCommand": strings.TrimSpace(listCommand),
		"kind":        strings.TrimSpace(selectors.kind),
		"matchTable":  strings.TrimSpace(matchTable),
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
	ambiguityErrTemplate = template.Must(template.New("").Parse("{{ .selectors }} matches multiple {{ .kind }}s:" + `

{{ .matchTable }}

Hint: use '{{ .listCommand }} -v' or '{{ .listCommand }} --format=[json|yaml]' to list all {{ .kind }}s with full details.
Hint: try selecting the {{ .kind }} with a more specific name (ex: {{ .command }} full-{{ .kind }}-name).
Hint: try selecting the {{ .kind }} with additional --labels or --query predicate.
`))
)
