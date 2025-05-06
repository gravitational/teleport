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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
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
	cf          *CLIConf
	tc          *client.TeleportClient
	client      databaseExecClient
	makeCommand func(context.Context, *databaseInfo, string, string) (*exec.Cmd, error)
	summary     databaseExecSummary
}

func newDatabaseExecCommand(cf *CLIConf) (*databaseExecCommand, error) {
	if err := checkDatabaseExecInputFlags(cf); err != nil {
		return nil, trace.Wrap(err)
	}

	tc, err := makeClient(cf)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sharedClient, err := newSharedDatabaseExecClient(cf, tc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	commandMaker, err := newDatabaseExecCommandMaker(cf.Context, tc, sharedClient.getProfileStatus())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &databaseExecCommand{
		cf:          cf,
		tc:          tc,
		client:      sharedClient,
		makeCommand: commandMaker.makeCommand,
	}, nil
}

func (c *databaseExecCommand) run() error {
	// Fetch.
	dbs, err := c.getDatabases()
	if err != nil {
		return trace.Wrap(err)
	}

	// Execute  in parallel.
	group, groupCtx := errgroup.WithContext(c.cf.Context)
	group.SetLimit(c.cf.ParallelJobs)
	for _, db := range dbs {
		group.Go(func() error {
			result := c.exec(groupCtx, db)
			c.summary.add(result)
			return nil
		})
	}

	// Print summary.
	defer func() {
		switch {
		case c.cf.OutputDir != "":
			c.summary.printAndSave(c.cf.Stdout(), c.cf.OutputDir)
		case len(dbs) > 1:
			c.summary.print(c.cf.Stdout())
		}
	}()

	return trace.Wrap(group.Wait())
}

func (c *databaseExecCommand) close() {
	if err := c.client.close(); err != nil {
		logger.WarnContext(c.cf.Context, "Failed to close client", "error", err)
	}
}

func checkDatabaseExecInputFlags(cf *CLIConf) error {
	// Pick an arbitrary number for max connections to avoid flooding the
	// backend. The limit can be overwritten with the "TELEPORT_PARALLEL_JOBS"
	// env var.
	const maxParallelJobs = 10
	if cf.ParallelJobs < 1 || cf.ParallelJobs > maxParallelJobs {
		return trace.BadParameter(`--parallel must be between 1 and %v`, maxParallelJobs)
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

	// Logging.
	if cf.ParallelJobs > 1 && cf.OutputDir == "" {
		return trace.BadParameter("--output-dir must be set when executing concurrent connections")
	}
	if cf.OutputDir != "" && utils.FileExists(cf.OutputDir) {
		return trace.BadParameter("directory %q already exists", cf.OutputDir)
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
	// Use a single predicate to search multiple names in one shot but batch 100
	// names at a time. Extra validation will be performed afterward to ensure
	// we fetched what we need.
	fmt.Fprintln(c.cf.Stdout(), "Fetching databases by name ...")

	// Trim spaces.
	names := stringFlagToStrings(c.cf.DatabaseServices)

	var dbs []types.Database
	for page := range slices.Chunk(names, 100) {
		var predicate string
		for _, name := range page {
			predicate = makePredicateDisjunction(predicate, makeDiscoveredNameOrNamePredicate(name))
		}

		logger.DebugContext(c.cf.Context, "Getting database by name", "databases", page)
		pageDBs, err := c.client.listDatabasesWithFilter(c.cf.Context, &proto.ListResourcesRequest{
			Namespace:           apidefaults.Namespace,
			ResourceType:        types.KindDatabaseServer,
			PredicateExpression: predicate,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		dbs = append(dbs, pageDBs...)
	}

	logger.DebugContext(c.cf.Context, "Fetched databases by name",
		"databases", logutils.IterAttr(types.ResourceNames(dbs)))
	return dbs, trace.Wrap(ensureEachDatabase(names, dbs))
}

func (c *databaseExecCommand) searchDatabases() (databases []types.Database, err error) {
	fmt.Fprintln(c.cf.Stdout(), "Searching databases ...")
	filter := c.tc.ResourceFilter(types.KindDatabaseServer)

	logger.DebugContext(c.cf.Context, "Searching for databases", "filter", filter)
	dbs, err := c.client.listDatabasesWithFilter(c.cf.Context, filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	logger.DebugContext(c.cf.Context, "Fetched databases with search filter",
		"databases", logutils.IterAttr(types.ResourceNames(dbs)),
	)
	return dbs, trace.Wrap(c.printSearchResultAndConfirm(dbs))
}

func (c *databaseExecCommand) printSearchResultAndConfirm(dbs []types.Database) error {
	if len(dbs) == 0 {
		return trace.NotFound("no databases found")
	}

	fmt.Fprintf(c.cf.Stdout(), "Found %d database(s):\n\n", len(dbs))
	printTableForDatabaseExec(c.cf.Stdout(), dbs)
	question := fmt.Sprintf("Do you want to proceed with %d database(s)?", len(dbs))
	if err := c.cf.PromptConfirmation(question); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *databaseExecCommand) exec(ctx context.Context, db types.Database) (result databaseExecResult) {
	result = databaseExecResult{
		RouteToDatabase: client.RouteToDatabaseToProto(c.makeRouteToDatabase(db)),
		Command:         c.cf.DatabaseCommand,
		Success:         true,
	}

	printErrorAndMakeErrorResult := func(err error) databaseExecResult {
		fmt.Fprintf(c.cf.Stderr(), "Failed to execute command for %q: %v\n", db.GetName(), err)
		result.Success = false
		result.Error = err.Error()
		return result
	}

	if ctx.Err() != nil {
		return printErrorAndMakeErrorResult(ctx.Err())
	}

	outputWriter := c.cf.Stdout()
	errWriter := c.cf.Stderr()
	switch {
	case c.cf.OutputDir != "":
		// Use full-name instead of display name for output path.
		logFile, err := c.openOutputFile(db.GetName())
		if err != nil {
			return printErrorAndMakeErrorResult(err)
		}
		defer logFile.Close()
		outputWriter = logFile
		errWriter = logFile
		fmt.Fprintf(c.cf.Stdout(), "Executing command for %q. Output will be saved at %q.\n", db.GetName(), logFile.Name())

		// Save absolute path in the summary. Not expecting the absolute check
		// to fail but use the filename in case it does.
		if result.OutputFile, err = filepath.Abs(logFile.Name()); err != nil {
			result.OutputFile = filepath.Base(logFile.Name())
		}
	default:
		// No prefix so output can still be copy-pasted. Extra empty line to
		// separate sequential executions.
		fmt.Fprintf(c.cf.Stdout(), "\nExecuting command for %q.\n", db.GetName())
	}

	var err error
	result.ExitCode, err = c.runCommand(ctx, db, outputWriter, errWriter)
	if err != nil {
		return printErrorAndMakeErrorResult(err)
	}
	return result
}

func (c *databaseExecCommand) runCommand(ctx context.Context, db types.Database, outputWriter, errWriter io.Writer) (int, error) {
	dbInfo, err := c.makeDatabaseInfo(db)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	lp, err := c.startLocalProxy(ctx, dbInfo)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	defer lp.Close()

	dbCmd, err := c.makeCommand(ctx, dbInfo, lp.GetAddr(), c.cf.DatabaseCommand)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	dbCmd.Stdout = outputWriter
	dbCmd.Stderr = errWriter

	logger.DebugContext(ctx, "Executing database command", "command", dbCmd, "db", dbInfo.ServiceName)
	runErr := c.cf.RunCommand(dbCmd)
	if dbCmd.ProcessState != nil {
		return dbCmd.ProcessState.ExitCode(), trace.Wrap(runErr)
	}
	return 0, trace.Wrap(runErr)
}

func (c *databaseExecCommand) startLocalProxy(ctx context.Context, dbInfo *databaseInfo) (*alpnproxy.LocalProxy, error) {
	// Issue single-use certificate.
	clientCert, err := c.client.issueCert(ctx, dbInfo)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Do not provide a re-issuer middleware to the local proxy. The local proxy
	// is meant for one-time use so there is no need to re-issue the
	// certificates.
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

func (c *databaseExecCommand) makeRouteToDatabase(db types.Database) tlsca.RouteToDatabase {
	return tlsca.RouteToDatabase{
		ServiceName: db.GetName(),
		Protocol:    db.GetProtocol(),
		Username:    c.cf.DatabaseUser,
		Database:    c.cf.DatabaseName,
		Roles:       requestedDatabaseRoles(c.cf),
	}
}

func (c *databaseExecCommand) makeDatabaseInfo(db types.Database) (*databaseInfo, error) {
	dbInfo := &databaseInfo{
		RouteToDatabase: c.makeRouteToDatabase(db),
		database:        db,
		checker:         c.client.getAccessChecker(),
	}
	return dbInfo, trace.Wrap(dbInfo.checkAndSetDefaults(c.cf, c.tc))
}

func (c *databaseExecCommand) outputFilename(dbServiceName string) string {
	return filepath.Join(c.cf.OutputDir, dbServiceName+".output")
}

func (c *databaseExecCommand) openOutputFile(dbServiceName string) (*os.File, error) {
	logFilePath, err := utils.EnsureLocalPath(c.outputFilename(dbServiceName), "", "")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	logFile, err := utils.CreateExclusiveFile(logFilePath, teleport.FileMaskOwnerOnly)
	return logFile, trace.ConvertSystemError(err)
}

// sharedDatabaseExecClient is a wrapper of client.TeleportClient that makes
// backend calls while using a shared ClusterClient.
type sharedDatabaseExecClient struct {
	profile       *client.ProfileStatus
	clusterClient *client.ClusterClient
	accessChecker services.AccessChecker
	tracer        oteltrace.Tracer

	issueCertMu         sync.Mutex
	reusableMFAResponse *proto.MFAAuthenticateResponse
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
		profile:       profile,
		tracer:        cf.TracingProvider.Tracer(teleport.ComponentTSH),
		clusterClient: clusterClient,
		accessChecker: accessChecker,
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

// issueCert issues a single use cert for the db route.
func (c *sharedDatabaseExecClient) issueCert(ctx context.Context, dbInfo *databaseInfo) (tls.Certificate, error) {
	c.issueCertMu.Lock()
	defer c.issueCertMu.Unlock()

	params := client.ReissueParams{
		RouteToDatabase:     client.RouteToDatabaseToProto(dbInfo.RouteToDatabase),
		AccessRequests:      c.profile.ActiveRequests,
		RequesterName:       proto.UserCertsRequest_TSH_DB_EXEC,
		ReusableMFAResponse: c.reusableMFAResponse,
	}

	result, err := c.clusterClient.IssueUserCertsWithMFA(ctx, params)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	// Save the reusable MFA response.
	if result.ReusableMFAResponse != nil {
		c.reusableMFAResponse = result.ReusableMFAResponse
	}

	dbCert, err := result.KeyRing.DBTLSCert(dbInfo.RouteToDatabase.ServiceName)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	return dbCert, nil
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

func (m *databaseExecCommandMaker) makeCommand(ctx context.Context, dbInfo *databaseInfo, lpAddr, command string) (*exec.Cmd, error) {
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
		GetExecCommand(ctx, command)
}

// ensureEachDatabase ensures one to one mapping between the provided database
// target names and database resources.
//
// Note that it is assumed that the provided database resource has at least one
// matching names as they are retrieved from the backend based on one of those
// names.
func ensureEachDatabase(names []string, dbs []types.Database) error {
	byDiscoveredNameOrName := map[string]types.Databases{}
	for _, db := range dbs {
		// Database may be listed by their original name in the cloud.
		byDiscoveredNameOrName[db.GetName()] = append(byDiscoveredNameOrName[db.GetName()], db)

		if discoveredName, ok := common.GetDiscoveredResourceName(db); ok && discoveredName != db.GetName() {
			byDiscoveredNameOrName[discoveredName] = append(byDiscoveredNameOrName[discoveredName], db)
		}
	}

	for _, name := range names {
		matched := byDiscoveredNameOrName[name]
		switch len(matched) {
		case 0:
			return trace.NotFound("database %q not found", name)
		case 1:
			continue
		default:
			var sb strings.Builder
			printTableForDatabaseExec(&sb, matched)
			return trace.BadParameter(`%q matches multiple databases:
%vTry selecting the database with a more specific name printed in the above table`, name, sb.String())
		}
	}

	return nil
}

func printTableForDatabaseExec(w io.Writer, dbs []types.Database) {
	rows := make([]databaseTableRow, 0, len(dbs))
	for _, db := range dbs {
		// Always use full name but don't print hidden labels.
		row := getDatabaseRow("", "", "", db, nil, nil, false)
		row.DisplayName = db.GetName()
		rows = append(rows, row)
	}
	printDatabaseTable(printDatabaseTableConfig{
		writer:         w,
		rows:           rows,
		includeColumns: []string{"Name", "Protocol", "Description", "Labels"},
	})
}

type databaseExecResult struct {
	proto.RouteToDatabase `json:"database"`
	Command               string `json:"command"`
	OutputFile            string `json:"output_file,omitempty"`
	Success               bool   `json:"success"`
	Error                 string `json:"error,omitempty"`
	ExitCode              int    `json:"exit_code"`
}

type databaseExecSummary struct {
	Databases []databaseExecResult `json:"databases"`
	Success   int                  `json:"success"`
	Failure   int                  `json:"failure"`
	Total     int                  `json:"total"`

	mu sync.Mutex
}

func (s *databaseExecSummary) add(result databaseExecResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Databases = append(s.Databases, result)
	s.Total++
	if result.Success {
		s.Success++
	} else {
		s.Failure++
	}
}

func (s *databaseExecSummary) print(w io.Writer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Fprintf(w, "\nSummary: %d of %d succeeded.\n", s.Success, s.Total)
}

func (s *databaseExecSummary) printAndSave(w io.Writer, outputDir string) {
	s.print(w)
	if err := s.save(w, outputDir); err != nil {
		fmt.Fprintf(w, "Failed to save summary: %v\n", err)
	}
}

func (s *databaseExecSummary) save(w io.Writer, outputDir string) error {
	summaryPath := filepath.Join(outputDir, "summary.json")
	summaryPath, err := utils.EnsureLocalPath(summaryPath, "", "")
	if err != nil {
		return trace.Wrap(err)
	}
	summaryFile, err := utils.CreateExclusiveFile(summaryPath, teleport.FileMaskOwnerOnly)
	if trace.IsAlreadyExists(err) {
		fmt.Fprintf(w, "Warning: file %s exists and will be overwritten.\n", summaryPath)
		summaryFile, err = os.OpenFile(summaryPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, teleport.FileMaskOwnerOnly)
		if err != nil {
			return trace.ConvertSystemError(err)
		}
	} else if err != nil {
		return trace.Wrap(err)
	}
	defer summaryFile.Close()

	summaryData, err := s.makeSummaryJSONData()
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := summaryFile.Write(summaryData); err != nil {
		return trace.ConvertSystemError(err)
	}

	fmt.Fprintf(w, "Summary is saved at %q.\n", summaryPath)
	return nil
}

func (s *databaseExecSummary) makeSummaryJSONData() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	summaryData, err := json.MarshalIndent(s, "", "  ")
	return summaryData, trace.Wrap(err)
}
