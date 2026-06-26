// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"context"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	libclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// AWSICCommand implements the "tctl awsic" group of commands, for inspecting
// AWS Identity Center resources synced into the cluster.
type AWSICCommand struct {
	// format is the output format.
	format string

	searchKeywords string

	// verbose sets whether full table output should be shown, including
	// permission set ARNs.
	verbose bool

	// accountsList implements the "tctl awsic ls" subcommand.
	accountsList *kingpin.CmdClause

	// stdout allows to switch the standard output source. Used in tests.
	stdout io.Writer
}

// Initialize allows AWSICCommand to plug itself into the CLI parser.
func (c *AWSICCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, _ *servicecfg.Config) {
	awsIC := app.Command("awsic", "Operate on AWS Identity Center resources synced with the cluster.")
	c.accountsList = awsIC.Command("ls", "List AWS Identity Center accounts and their permission sets.")
	c.accountsList.Flag("format", "Output format, 'text', 'json', or 'yaml'").Default(teleport.Text).StringVar(&c.format)
	c.accountsList.Flag("verbose", "Verbose table output, shows permission set ARNs (one row per permission set)").Short('v').BoolVar(&c.verbose)

	if c.stdout == nil {
		c.stdout = os.Stdout
	}
}

// TryRun attempts to run subcommands like "awsic ls".
func (c *AWSICCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(ctx context.Context, client *authclient.Client) error
	switch cmd {
	case c.accountsList.FullCommand():
		commandFunc = c.ListAccounts
	default:
		return false, nil
	}
	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	err = commandFunc(ctx, client)
	closeFn(ctx)
	return true, trace.Wrap(err)
}

// ListAccounts prints the AWS Identity Center accounts synced into the cluster
// along with their permission sets.
func (c *AWSICCommand) ListAccounts(ctx context.Context, clt *authclient.Client) error {
	resources, err := apiclient.GetAllUnifiedResources(ctx, clt, &proto.ListUnifiedResourcesRequest{
		Kinds:          []string{types.KindApp},
		SearchKeywords: libclient.ParseSearchKeywords(c.searchKeywords, ','),
		// This will list all permission sets including ones that the user
		// is NOT able to assume. This is intentional so that user can see
		// what is available.
		IncludeLogins: false,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	servers := filterICAccounts(resources)

	switch c.format {
	case teleport.Text:
		return trace.Wrap(c.writeText(c.stdout, servers))
	case teleport.JSON:
		return trace.Wrap(utils.WriteJSONArray(c.stdout, servers))
	case teleport.YAML:
		return trace.Wrap(utils.WriteYAML(c.stdout, servers))
	default:
		return trace.BadParameter("unknown format %q", c.format)
	}
}

func filterICAccounts(resources []*types.EnrichedResource) []types.AppServer {
	var servers []types.AppServer
	for _, r := range resources {
		appServer, ok := r.ResourceWithLabels.(types.AppServer)
		if !ok {
			continue
		}
		if appServer.GetApp().GetIdentityCenter() == nil {
			continue
		}
		servers = append(servers, appServer)
	}

	sort.Slice(servers, func(i, j int) bool {
		return icAccountName(servers[i].GetApp()) < icAccountName(servers[j].GetApp())
	})

	return servers
}

func (c *AWSICCommand) writeText(w io.Writer, servers []types.AppServer) error {
	if c.verbose {
		// One row per permission set so the full ARN is readable and not truncated.
		headers := []string{"Account Name", "Account ID", "Permission Set", "Permission Set ARN"}
		rows := make([][]string, 0, len(servers))
		for _, server := range servers {
			app := server.GetApp()
			ic := app.GetIdentityCenter()
			name := icAccountName(app)
			if len(ic.PermissionSets) == 0 {
				rows = append(rows, []string{name, ic.AccountID, "", ""})
				continue
			}
			for _, ps := range ic.PermissionSets {
				rows = append(rows, []string{name, ic.AccountID, ps.Name, ps.ARN})
			}
		}
		t := asciitable.MakeTable(headers, rows...)
		_, err := t.AsBuffer().WriteTo(w)
		return trace.Wrap(err)
	}

	headers := []string{"Account Name", "Account ID", "Permission Sets"}
	rows := make([][]string, 0, len(servers))
	for _, server := range servers {
		app := server.GetApp()
		ic := app.GetIdentityCenter()
		psNames := make([]string, 0, len(ic.PermissionSets))
		for _, ps := range ic.PermissionSets {
			psNames = append(psNames, ps.Name)
		}
		rows = append(rows, []string{icAccountName(app), ic.AccountID, strings.Join(psNames, ", ")})
	}
	t := asciitable.MakeTableWithTruncatedColumn(headers, rows, "Permission Sets")
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

// icAccountName returns human-readable name for an Identity Center account
// (versus the AWS account ID).
func icAccountName(app types.Application) string {
	if name := types.FriendlyName(app); name != "" {
		return name
	}
	if name, ok := app.GetLabel(types.AWSAccountNameLabel); ok && name != "" {
		return name
	}
	if ic := app.GetIdentityCenter(); ic != nil {
		return ic.AccountID
	}
	return ""
}
