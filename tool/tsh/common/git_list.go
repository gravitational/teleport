/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"fmt"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
)

// gitListCommand implements `tsh git ls`.
type gitListCommand struct {
	*kingpin.CmdClause

	format              string
	labels              string
	predicateExpression string
	searchKeywords      string

	// fetchFn is the function to fetch git servers. Defaults to c.doFetch.
	// Can be set for testing.
	fetchFn func(*CLIConf, *client.TeleportClient) ([]types.Server, error)
}

func newGitListCommand(parent *kingpin.CmdClause) *gitListCommand {
	cmd := &gitListCommand{
		CmdClause: parent.Command("ls", "List Git servers."),
	}
	cmd.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)).
		Short('f').
		Default(teleport.Text).
		EnumVar(&cmd.format, defaults.DefaultFormats...)

	cmd.Flag("search", searchHelp).StringVar(&cmd.searchKeywords)
	cmd.Flag("query", queryHelp).StringVar(&cmd.predicateExpression)
	cmd.Arg("labels", labelHelp).StringVar(&cmd.labels)
	return cmd
}

func (c *gitListCommand) run(cf *CLIConf) error {
	c.init(cf)

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	servers, err := c.fetchFn(cf, tc)
	if err != nil {
		return trace.Wrap(err)
	}
	return printGitServers(cf, servers)
}

func (c *gitListCommand) init(cf *CLIConf) {
	cf.Format = c.format
	cf.Labels = c.labels
	cf.PredicateExpression = c.predicateExpression
	cf.SearchKeywords = c.searchKeywords

	if c.fetchFn == nil {
		c.fetchFn = c.doFetch
	}
}

func (c *gitListCommand) doFetch(cf *CLIConf, tc *client.TeleportClient) ([]types.Server, error) {
	var resources types.EnrichedResources
	err := client.RetryWithRelogin(cf.Context, tc, func() error {
		client, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer client.Close()

		resources, err = apiclient.GetAllUnifiedResources(cf.Context, client.AuthClient, &proto.ListUnifiedResourcesRequest{
			Kinds:               []string{types.KindGitServer},
			SortBy:              types.SortBy{Field: types.ResourceMetadataName},
			SearchKeywords:      tc.SearchKeywords,
			PredicateExpression: tc.PredicateExpression,
		})
		return trace.Wrap(err)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resources.ToResourcesWithLabels().AsServers()
}

func printGitServers(cf *CLIConf, servers []types.Server) error {
	format := strings.ToLower(cf.Format)
	switch format {
	case teleport.Text, "":
		return printGitServersAsText(cf, servers)
	case teleport.JSON, teleport.YAML:
		out, err := serializeNodes(servers, format)
		if err != nil {
			return trace.Wrap(err)
		}
		if _, err := fmt.Fprintln(cf.Stdout(), out); err != nil {
			return trace.Wrap(err)
		}
		return nil
	default:
		return trace.BadParameter("unsupported format %q", format)
	}
}

func printGitServersAsText(cf *CLIConf, servers []types.Server) error {
	var rows [][]string
	var showLoginNote bool
	profileStatus, err := cf.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}
	for _, server := range servers {
		login := "(n/a)*"

		if github := server.GetGitHub(); github != nil {
			if profileStatus.GitHubIdentity != nil {
				login = profileStatus.GitHubIdentity.Username
			} else {
				showLoginNote = true
			}

			rows = append(rows, []string{
				"GitHub",
				github.Organization,
				login,
				github.GetOrganizationURL(),
			})
		} else {
			return trace.BadParameter("expecting Git server but got %v", server.GetKind())
		}
	}

	t := asciitable.MakeTable([]string{"Type", "Organization", "Username", "URL"}, rows...)
	if _, err := fmt.Fprintln(cf.Stdout(), t.AsBuffer().String()); err != nil {
		return trace.Wrap(err)
	}

	if showLoginNote {
		fmt.Fprint(cf.Stdout(), gitLoginNote)
	}

	fmt.Fprint(cf.Stdout(), gitCommandsGeneralHint)
	return nil
}

const gitLoginNote = "" +
	"(n/a)*: Username will be retrieved automatically upon running git commands.\n" +
	"        Alternatively, run `tsh git login --github-org <org>`.\n\n"

const gitCommandsGeneralHint = "" +
	"hint: use 'tsh git clone <git-clone-ssh-url>' to clone a new repository\n" +
	"      use 'tsh git config update' to configure an existing repository to use Teleport\n" +
	"      once the repository is cloned or configured, use 'git' as normal\n\n"
