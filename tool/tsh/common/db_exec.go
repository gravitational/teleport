/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	"github.com/schollz/progressbar/v3"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/db/dbcmd"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/tool/common"
)

func onDatabaseExec(cf *CLIConf) error {
	execCommand, err := newDatabaseExecCommand(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	defer execCommand.close()

	return trace.Wrap(execCommand.run())
}

// databaseExecClient is a wrapper of client.TeleportClient that makes backend
// calls. can be mocked for testing.
type databaseExecClient interface {
	close() error
	getProfileStatus() *client.ProfileStatus
	getAccessChecker() services.AccessChecker
	issueCert(context.Context, *databaseInfo) (tls.Certificate, error)
	listDatabasesWithFilter(context.Context, *proto.ListResourcesRequest) ([]types.Database, error)
}

type databaseExecCommand struct {
	cf                     *CLIConf
	tc                     *client.TeleportClient
	client                 databaseExecClient
	makeCommand            func(context.Context, *databaseInfo, string, string) (*exec.Cmd, error)
	prefixedOutputHintOnce sync.Once
}

func newDatabaseExecCommand(cf *CLIConf) (*databaseExecCommand, error) {
	err := checkDatabaseExecInputFlags(cf)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c := new(databaseExecCommand)
	c.tc, err = makeClient(cf)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sharedClient, err := newSharedDatabaseExecClient(cf, c.tc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.client = sharedClient

	commandMaker, err := newDatabaseExecCommandMaker(cf.Context, c.tc, c.client.getProfileStatus())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.makeCommand = commandMaker.makeCommand
	return c, nil
}

func (c *databaseExecCommand) run() error {
	// Fetch.
	dbs, err := c.getDatabases()
	if err != nil {
		return trace.Wrap(err)
	}

	// Pre-checks before execution.
	for _, db := range dbs {
		if err := c.checkDatabase(db); err != nil {
			return trace.Wrap(err)
		}
	}

	// Execute queries in parallel.
	group, groupCtx := errgroup.WithContext(c.cf.Context)
	group.SetLimit(c.cf.MaxConnections)
	for _, db := range dbs {
		group.Go(func() error {
			return trace.Wrap(c.exec(groupCtx, db))
		})
	}
	return trace.Wrap(group.Wait())
}

func (c *databaseExecCommand) close() {
	if err := c.client.close(); err != nil {
		logger.WarnContext(c.cf.Context, "Failed to close client", "error", err)
	}
}

func checkDatabaseExecInputFlags(cf *CLIConf) error {
	// Pick an arbitrary number to avoid flooding the backend.
	if cf.MaxConnections < 1 || cf.MaxConnections > 10 {
		return trace.BadParameter("--max-connections must be between 1 and 10")
	}

	// Selection flags.
	byNames := cf.DatabaseServices != ""
	bySearch := cf.SearchKeywords != "" || cf.Labels != ""
	switch {
	case !byNames && !bySearch:
		return trace.BadParameter("please provide one of --dbs, --labels, --search flags")
	case byNames && bySearch:
		return trace.BadParameter("--labels/--search flags cannot be used with --dbs flag")
	}
	return nil
}

func (c *databaseExecCommand) getDatabases() ([]types.Database, error) {
	if c.cf.DatabaseServices != "" {
		return c.getDatabasesByNames()
	}
	return c.searchDatabases()
}

func (c *databaseExecCommand) getDatabasesByNames() ([]types.Database, error) {
	names := apiutils.Deduplicate(strings.Split(c.cf.DatabaseServices, ","))

	// Show a progress bar when fetching more than one database.
	var progress *progressbar.ProgressBar
	if len(names) > 1 {
		fmt.Fprintln(c.cf.Stdout(), "Fetching databases by names:")
		progress = progressbar.NewOptions(
			len(names),
			progressbar.OptionSetWriter(c.cf.Stdout()),
			progressbar.OptionShowCount(),
			progressbar.OptionSetElapsedTime(false),
			progressbar.OptionSetPredictTime(false),
		)
		defer func() {
			// Print a break line for the progress bar. Note that progress.Close
			// is not called to avoid filling the progress bar on errors.
			fmt.Fprintln(c.cf.Stdout(), "")
		}()
	}

	// Fetch in parallel.
	group, groupCtx := errgroup.WithContext(c.cf.Context)
	group.SetLimit(c.cf.MaxConnections)
	var (
		mu  sync.Mutex
		dbs []types.Database
	)
	for _, name := range names {
		group.Go(func() error {
			logger.DebugContext(c.cf.Context, "Getting database by name", "name", name)
			list, err := c.client.listDatabasesWithFilter(groupCtx, &proto.ListResourcesRequest{
				Namespace:           apidefaults.Namespace,
				ResourceType:        types.KindDatabaseServer,
				PredicateExpression: makeDiscoveredNameOrNamePredicate(name),
			})
			if err != nil {
				return trace.Wrap(err)
			}
			switch len(list) {
			case 0:
				return trace.NotFound("database %q not found", name)
			case 1:
				mu.Lock()
				defer mu.Unlock()
				dbs = append(dbs, list[0])
				if progress != nil {
					progress.Add(1)
				}
				return nil
			default:
				return trace.CompareFailed("expecting one database but got %d", len(list))
			}
		})
	}

	if err := group.Wait(); err != nil {
		return nil, trace.Wrap(err)
	}

	logger.DebugContext(c.cf.Context, "Fetched database services by names.",
		"databases", logutils.IterAttr(types.ResourceNameIter(dbs)))
	return dbs, nil
}

func (c *databaseExecCommand) searchDatabases() (databases []types.Database, err error) {
	filter := c.tc.ResourceFilter(types.KindDatabaseServer)
	logger.DebugContext(c.cf.Context, "Searching for databases", "filter", filter)

	dbs, err := c.client.listDatabasesWithFilter(c.cf.Context, filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	logger.DebugContext(c.cf.Context, "Fetched databases with search filter.",
		"databases", logutils.IterAttr(types.ResourceNameIter(dbs)),
	)

	if len(dbs) == 0 {
		return nil, trace.NotFound("no databases found")
	}

	// Print results and prompt for confirmation.
	fmt.Fprintf(c.cf.Stdout(), "Found %d database(s):\n\n", len(dbs))
	var rows []databaseTableRow
	for _, db := range dbs {
		rows = append(rows, getDatabaseRow("", "", "", db, nil, nil, false))
	}
	printDatabaseTable(printDatabaseTableConfig{
		writer:         c.cf.Stdout(),
		rows:           rows,
		includeColumns: []string{"Name", "Protocol", "Description", "Labels"},
	})

	if err := c.cf.PromptConfirmation("Do you want to proceed?"); err != nil {
		return nil, trace.Wrap(err)
	}
	return dbs, nil
}

func (c *databaseExecCommand) checkDatabase(db types.Database) error {
	switch db.GetProtocol() {
	case types.DatabaseProtocolPostgreSQL, types.DatabaseProtocolMySQL:
	default:
		return trace.NotImplemented("unsupported database protocol: %s", db.GetProtocol())
	}

	return trace.Wrap(c.makeDatabaseInfo(db).checkAndSetDefaults(c.cf, c.tc))
}

func (c *databaseExecCommand) exec(ctx context.Context, db types.Database) (err error) {
	displayName := common.FormatResourceName(db, false)
	outputWriter := c.cf.Stdout()
	errWriter := c.cf.Stderr()
	defer func() {
		// Print the error and return nil to continue-on-error.
		if err != nil {
			fmt.Fprintln(errWriter, err)
			if c.cf.OutputDir != "" {
				fmt.Fprintf(c.cf.Stderr(), "Failed to execute command for %q. See output file for more details.\n", displayName)
			}
			err = nil
		}
	}()

	switch {
	case c.cf.OutputDir != "":
		// Use full-name instead of display name for output path.
		logFile, err := c.openOutputFile(db.GetName())
		if err != nil {
			return trace.Wrap(err)
		}
		outputWriter = logFile
		errWriter = logFile
		fmt.Fprintf(c.cf.Stdout(), "Executing command for %q. Output will be saved at %q.\n", displayName, logFile.Name())
	case c.cf.MaxConnections > 1:
		outputWriterWithPrefix := newDBPrefixWriter(c.cf.Stdout(), displayName)
		errWriterWithPrefix := newDBPrefixWriter(c.cf.Stderr(), displayName)
		defer outputWriterWithPrefix.Close()
		defer errWriterWithPrefix.Close()
		outputWriter = outputWriterWithPrefix
		errWriter = errWriterWithPrefix
		c.prefixedOutputHintOnce.Do(func() {
			fmt.Fprintf(c.cf.Stdout(), `Outputs will be prefixed with the name of the target database.
Alternatively, use --output-dir flag to save the outputs to files.
`)
		})
		fmt.Fprintf(c.cf.Stdout(), "Executing command for %q.\n", displayName)
	default:
		// No prefix so output can still be copy-pasted. Extra empty line to
		// separate sequential executions.
		fmt.Fprintf(c.cf.Stdout(), "\nExecuting command for %q.\n", displayName)
	}

	dbInfo := c.makeDatabaseInfo(db)
	lp, err := c.startLocalProxy(ctx, dbInfo)
	if err != nil {
		return trace.Wrap(err)
	}
	defer lp.Close()

	dbCmd, err := c.makeCommand(ctx, dbInfo, lp.GetAddr(), c.cf.DatabaseQuery)
	if err != nil {
		return trace.Wrap(err)
	}
	dbCmd.Stdout = outputWriter
	dbCmd.Stderr = errWriter

	logger.DebugContext(ctx, "Executing database command", "command", dbCmd, "db", db.GetName())
	return trace.Wrap(c.cf.RunCommand(dbCmd))
}

func (c *databaseExecCommand) startLocalProxy(ctx context.Context, dbInfo *databaseInfo) (*alpnproxy.LocalProxy, error) {
	// Issue a single use cert, and do not provide a re-issuer middleware to the
	// local proxy.
	clientCert, err := c.client.issueCert(ctx, dbInfo)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listener, err := createLocalProxyListener("localhost:0", dbInfo.RouteToDatabase, c.client.getProfileStatus())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	opts := []alpnproxy.LocalProxyConfigOpt{
		alpnproxy.WithDatabaseProtocol(dbInfo.Protocol),
		alpnproxy.WithClusterCAsIfConnUpgrade(ctx, c.tc.RootClusterCACertPool),
		alpnproxy.WithClientCert(clientCert),
	}

	lpConfig := makeBasicLocalProxyConfig(ctx, c.tc, listener, c.cf.InsecureSkipVerify)
	lp, err := alpnproxy.NewLocalProxy(lpConfig, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	go func() {
		defer listener.Close()
		if err := lp.Start(ctx); err != nil {
			logger.ErrorContext(ctx, "Failed to start local proxy", "error", err)
		}
	}()
	return lp, nil
}

func (c *databaseExecCommand) makeDatabaseInfo(db types.Database) *databaseInfo {
	return &databaseInfo{
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: db.GetName(),
			Protocol:    db.GetProtocol(),
			Username:    c.cf.DatabaseUser,
			Database:    c.cf.DatabaseName,
			Roles:       requestedDatabaseRoles(c.cf),
		},
		database: db,
		checker:  c.client.getAccessChecker(),
	}
}

func (c *databaseExecCommand) openOutputFile(dbServiceName string) (*os.File, error) {
	logFilePath := filepath.Join(c.cf.OutputDir, dbServiceName+".output")
	logFilePath, err := utils.EnsureLocalPath(logFilePath, "", "")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	logFile, err := os.Create(logFilePath)
	return logFile, trace.ConvertSystemError(err)
}

// sharedDatabaseExecClient is a wrapper of client.TeleportClient that makes
// backend calls while using a shared ClusterClient.
type sharedDatabaseExecClient struct {
	*client.TeleportClient
	profile       *client.ProfileStatus
	clusterClient *client.ClusterClient
	accessChecker services.AccessChecker
	tracer        oteltrace.Tracer
}

func newSharedDatabaseExecClient(cf *CLIConf, tc *client.TeleportClient) (*sharedDatabaseExecClient, error) {
	var clusterClient *client.ClusterClient
	var err error
	if err := client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err = tc.ConnectToCluster(cf.Context)
		return trace.Wrap(err)
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	profile, err := tc.ProfileStatus()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessChecker, err := services.NewAccessCheckerForRemoteCluster(cf.Context, profile.AccessInfo(), tc.SiteName, clusterClient.AuthClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &sharedDatabaseExecClient{
		TeleportClient: tc,
		profile:        profile,
		tracer:         cf.TracingProvider.Tracer(teleport.ComponentTSH),
		clusterClient:  clusterClient,
		accessChecker:  accessChecker,
	}, nil
}

func (c *sharedDatabaseExecClient) close() error {
	if err := c.clusterClient.Close(); err != nil && !trace.IsConnectionProblem(err) {
		return trace.Wrap(err)
	}
	return nil
}

func (c *sharedDatabaseExecClient) getAccessChecker() services.AccessChecker {
	return c.accessChecker
}

func (c *sharedDatabaseExecClient) getProfileStatus() *client.ProfileStatus {
	return c.profile
}

func (c *sharedDatabaseExecClient) issueCert(ctx context.Context, dbInfo *databaseInfo) (tls.Certificate, error) {
	// TODO(greedy52) add support for multi-session MFA.
	params := client.ReissueParams{
		RouteToCluster:  c.SiteName,
		RouteToDatabase: client.RouteToDatabaseToProto(dbInfo.RouteToDatabase),
		AccessRequests:  c.profile.ActiveRequests,
	}

	keyRing, _, err := c.clusterClient.IssueUserCertsWithMFA(ctx, params)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	dbCert, err := keyRing.DBTLSCert(dbInfo.RouteToDatabase.ServiceName)
	return dbCert, trace.Wrap(err)
}

func (c *sharedDatabaseExecClient) listDatabasesWithFilter(ctx context.Context, filter *proto.ListResourcesRequest) (databases []types.Database, err error) {
	ctx, span := c.tracer.Start(
		ctx,
		"listDatabasesWithFilter",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	servers, err := apiclient.GetAllResources[types.DatabaseServer](ctx, c.clusterClient.AuthClient, filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return types.DatabaseServers(servers).ToDatabases(), nil
}

type databaseExecCommandMaker struct {
	tc          *client.TeleportClient
	profile     *client.ProfileStatus
	rootCluster string
}

func newDatabaseExecCommandMaker(ctx context.Context, tc *client.TeleportClient, profile *client.ProfileStatus) (*databaseExecCommandMaker, error) {
	rootCluster, err := tc.RootClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &databaseExecCommandMaker{
		tc:          tc,
		profile:     profile,
		rootCluster: rootCluster,
	}, nil
}

func (m *databaseExecCommandMaker) makeCommand(ctx context.Context, dbInfo *databaseInfo, lpAddr, execQuery string) (*exec.Cmd, error) {
	addr, err := utils.ParseAddr(lpAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	opts, err := makeDatabaseCommandOptions(ctx, m.tc, dbInfo,
		dbcmd.WithLocalProxy("localhost", addr.Port(0), ""),
		dbcmd.WithNoTLS(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return dbcmd.NewCmdBuilder(m.tc, m.profile, dbInfo.RouteToDatabase, m.rootCluster, opts...).
		GetExecCommand(ctx, execQuery)
}
