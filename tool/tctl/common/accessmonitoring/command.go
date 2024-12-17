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

package accessmonitoring

import (
	"context"
	"io"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"golang.org/x/exp/maps"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/secreports"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// Command implements `tctl audit` group of commands.
type Command struct {
	handler     cmdHandler
	innerCmdMap map[string]runFunc
}

// Initialize allows to implement Command interface.
func (c *Command) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, cfg *servicecfg.Config) {
	c.innerCmdMap = map[string]runFunc{}

	auditCmd := app.Command("audit", "Audit command.")
	auditCmd.Flag("days", "Days range (default 7)").Default("7").IntVar(&c.handler.days)
	auditCmd.Flag("format", "Output format: 'yaml' or 'json'").Default(teleport.YAML).StringVar(&c.handler.format)
	c.initAuditQueryCommands(auditCmd, cfg)
	c.initAuditReportsCommands(auditCmd, cfg)
}

type cmdHandler struct {
	name       string
	days       int
	auditQuery string
	format     string
}

func (c *Command) initAuditQueryCommands(auditCmd *kingpin.CmdClause, cfg *servicecfg.Config) {
	query := auditCmd.Command("query", "Audit query.")
	getCmd := query.Command("get", "Get audit query.")
	getCmd.Arg("name", "name of the audit query").Required().StringVar(&c.handler.name)

	rmCmd := query.Command("rm", "Remove audit query.")
	rmCmd.Arg("name", "name of the audit query").Required().StringVar(&c.handler.name)

	lsCmd := query.Command("ls", "List audit queries.")

	execCmd := query.Command("exec", "Execute audit query.")
	execCmd.Arg("query", "SQL Query").StringVar(&c.handler.auditQuery)

	schemaCmd := auditCmd.Command("schema", "Print audit query schema.")

	createCmd := query.Command("create", "Create an audit query.")
	createCmd.Arg("query", "SQL Query").StringVar(&c.handler.auditQuery)
	createCmd.Flag("name", "Audit query name").StringVar(&c.handler.name)

	maps.Copy(c.innerCmdMap, map[string]runFunc{
		execCmd.FullCommand():   c.handler.onAuditQueryExec,
		getCmd.FullCommand():    c.handler.onAuditQueryGet,
		lsCmd.FullCommand():     c.handler.onAuditQueryLs,
		rmCmd.FullCommand():     c.handler.onAuditQueryRm,
		schemaCmd.FullCommand(): c.handler.onAuditQuerySchema,
		createCmd.FullCommand(): c.handler.onAuditQueryCreate,
	})
}

func (c *Command) initAuditReportsCommands(auditCmd *kingpin.CmdClause, cfg *servicecfg.Config) {
	reportCmd := auditCmd.Command("report", "Access Monitoring related commands.")

	lsCmd := reportCmd.Command("ls", "List security reports.")

	getCmd := reportCmd.Command("get", "Get security report.")
	getCmd.Arg("name", "security name").Required().StringVar(&c.handler.name)

	runCmd := reportCmd.Command("run", "Run the security report.")
	runCmd.Arg("name", "security report name").Required().StringVar(&c.handler.name)

	stateCmd := reportCmd.Command("state", "Print the state of the security report.")
	stateCmd.Arg("name", "security report name").Required().StringVar(&c.handler.name)

	maps.Copy(c.innerCmdMap, map[string]runFunc{
		lsCmd.FullCommand():    c.handler.onAuditReportLs,
		getCmd.FullCommand():   c.handler.onAuditReportGet,
		runCmd.FullCommand():   c.handler.onAuditReportRun,
		stateCmd.FullCommand(): c.handler.onAuditReportState,
	})
}

type runFunc func(context.Context, *authclient.Client) error

func (c *Command) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	handler, ok := c.innerCmdMap[cmd]
	if !ok {
		return false, nil
	}

	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	defer closeFn(ctx)

	switch err := trail.FromGRPC(handler(ctx, client)); {
	case trace.IsNotImplemented(err):
		return true, trace.AccessDenied("Access Monitoring requires a Teleport Enterprise Auth Server.")
	default:
		return true, trace.Wrap(err)
	}
}

func (c *cmdHandler) onAuditQueryExec(ctx context.Context, authClient *authclient.Client) error {
	if c.auditQuery == "" {
		buff, err := io.ReadAll(os.Stdin)
		if err != nil {
			return trace.Wrap(err)
		}
		c.auditQuery = string(buff)
	}
	resp, err := authClient.SecReportsClient().RunAuditQueryAndGetResult(ctx, c.auditQuery, c.days)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := utils.WriteJSON(os.Stdout, resp); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *cmdHandler) onAuditQueryGet(ctx context.Context, authClient *authclient.Client) error {
	auditQuery, err := authClient.SecReportsClient().GetSecurityAuditQuery(ctx, c.name)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := printResource(auditQuery, c.format); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *cmdHandler) onAuditQueryLs(ctx context.Context, authClient *authclient.Client) error {
	auditQueries, err := authClient.SecReportsClient().GetSecurityAuditQueries(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := printResource(auditQueries, c.format); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *cmdHandler) onAuditQueryRm(ctx context.Context, authClient *authclient.Client) error {
	if err := authClient.SecReportsClient().DeleteSecurityAuditQuery(ctx, c.name); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *cmdHandler) onAuditQuerySchema(ctx context.Context, authClient *authclient.Client) error {
	resp, err := authClient.SecReportsClient().GetSchema(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, view := range resp.GetViews() {
		table := asciitable.MakeTable([]string{"Name", "Type", "Description"})
		for _, v := range view.Columns {
			table.AddRow([]string{v.Name, v.Type, v.Desc})
		}
		_, err = table.AsBuffer().WriteTo(os.Stdout)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *cmdHandler) onAuditQueryCreate(ctx context.Context, authClient *authclient.Client) error {
	if c.auditQuery == "" {
		return trace.BadParameter("audit query required")
	}
	if c.name == "" {
		return trace.BadParameter("audit query name required")
	}
	res, err := secreports.NewAuditQuery(header.Metadata{Name: c.name}, secreports.AuditQuerySpec{
		Query: c.auditQuery,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	err = authClient.SecReportsClient().UpsertSecurityAuditQuery(ctx, res)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *cmdHandler) onAuditReportLs(ctx context.Context, authClient *authclient.Client) error {
	reports, err := authClient.SecReportsClient().GetSecurityReports(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := printResource(reports, c.format); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(err)
}

func (c *cmdHandler) onAuditReportGet(ctx context.Context, authClient *authclient.Client) error {
	details, err := authClient.SecReportsClient().GetSecurityReportResult(ctx, c.name, c.days)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := printResource(details, c.format); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *cmdHandler) onAuditReportRun(ctx context.Context, authClient *authclient.Client) error {
	err := authClient.SecReportsClient().RunSecurityReport(ctx, c.name, c.days)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *cmdHandler) onAuditReportState(ctx context.Context, authClient *authclient.Client) error {
	state, err := authClient.SecReportsClient().GetSecurityReportExecutionState(ctx, c.name, int32(c.days))
	if err != nil {
		return trace.Wrap(err)
	}
	if err := printResource(state, c.format); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func printResource(resource any, format string) error {
	switch format {
	case teleport.JSON:
		if err := utils.WriteJSON(os.Stdout, resource); err != nil {
			return trace.Wrap(err)
		}
	case teleport.YAML:
		if err := utils.WriteYAML(os.Stdout, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		return trace.BadParameter("unsupported output format %s, supported values are %s and %s", format, teleport.JSON, teleport.YAML)
	}
	return nil
}
