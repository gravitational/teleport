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
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/prompt"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/trace"
)

func onGitClone(cf *CLIConf) error {
	// TODO validate host and do a git_server fetch before cloning.
	_, org, err := parseGitURL(strings.TrimSpace(cf.GitURL))
	if err != nil {
		return trace.Wrap(err)
	}

	config := fmt.Sprintf("core.sshcommand=%s", makeGitSSHCommand(org))
	cmd := exec.CommandContext(cf.Context, "git", "clone", "--config", config, cf.GitURL)
	cmd.Stdin = cf.Stdin()
	cmd.Stdout = cf.Stdout()
	cmd.Stderr = cf.Stderr()
	return trace.Wrap(cmd.Run())
}

func onGitSSH(cf *CLIConf) error {
	maybeUseTTYAsStdinFallback(cf.Context)

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO validate the host is github.com
	user, err := getGitHubUserID(cf, tc)
	if err != nil {
		return trace.Wrap(err)
	}

	cf.UserHost = fmt.Sprintf("%s@%s", user, types.MakeGitHubOrgServerDomain(cf.GitHubOrg))
	log.Debugf("=== onGitSSH org %s userhost %s command %s", cf.GitHubOrg, cf.UserHost, cf.RemoteCommand)

	// Make again to reflect cf.UserHost change // TODO improve perf
	tc, err = makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	tc.Stdin = os.Stdin
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		return tc.SSH(cf.Context, cf.RemoteCommand)
	})
	// Exit with the same exit status as the failed command.
	return trace.Wrap(convertSSHExitCode(tc, err))
}

func onGitConfig(cf *CLIConf) error {
	switch cf.GitConfigAction {
	case "":
		return trace.Wrap(onGitConfigCheck(cf))
	case "update":
		return trace.Wrap(onGitConfigUpdate(cf))
	case "reset":
		return trace.Wrap(onGitConfigReset(cf))
	default:
		return trace.BadParameter("unknown option for git config")
	}
}

func onGitConfigCheck(cf *CLIConf) error {
	var bufStd bytes.Buffer
	cmd := exec.CommandContext(cf.Context, "git", "config", "--local", "--default", "", "--get", "core.sshcommand")
	cmd.Stdout = &bufStd
	cmd.Stderr = cf.Stderr()

	if err := cmd.Run(); err != nil {
		return trace.Wrap(err)
	}

	output := strings.TrimSpace(bufStd.String())
	wantPrefix := makeGitSSHCommand("")
	switch {
	case strings.HasPrefix(output, wantPrefix):
		_, org, _ := strings.Cut(output, wantPrefix)
		fmt.Fprintf(cf.Stdout(), "The current git dir is configured with Teleport for GitHub organization %q.\n\n", org)
		cf.GitHubOrg = org
	default:
		fmt.Fprintf(cf.Stdout(), "The current git dir is not configured with Teleport.\n"+
			"Run 'tsh git config udpate' to configure it.\n\n")
		return nil
	}

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	username, err := getGitHubUserID(cf, tc)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintf(cf.Stdout(), "Your GitHub username is %q.\n", username)
	return nil
}

func onGitConfigUpdate(cf *CLIConf) error {
	var bufStd bytes.Buffer
	lsCmd := exec.CommandContext(cf.Context, "git", "ls-remote", "--get-url")
	lsCmd.Stdout = &bufStd
	lsCmd.Stderr = cf.Stderr()
	if err := lsCmd.Run(); err != nil {
		return trace.Wrap(err)
	}

	for _, gitURL := range strings.Split(bufStd.String(), "\n") {
		host, org, err := parseGitURL(strings.TrimSpace(gitURL))
		if err != nil {
			return trace.Wrap(err)
		}
		if host == "github.com" {
			cmd := exec.CommandContext(cf.Context, "git", "config", "--local", "--replace-all", "core.sshcommand", makeGitSSHCommand(org))
			cmd.Stdout = cf.Stdout()
			cmd.Stderr = cf.Stderr()
			return trace.NewAggregate(cmd.Run(), onGitConfigCheck(cf))
		}
	}
	return trace.NotFound("no supported git url found from 'git ls-remote --get-url': %s", bufStd.String())
}

func onGitConfigReset(cf *CLIConf) error {
	// TODO do we want to verify the current content be fore reset
	cmd := exec.CommandContext(cf.Context, "git", "config", "--local", "--unset-all", "core.sshcommand")
	cmd.Stdout = cf.Stdout()
	cmd.Stderr = cf.Stderr()
	return trace.NewAggregate(cmd.Run(), onGitConfigCheck(cf))
}

func onGitList(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	var resources []*types.EnrichedResource
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		client, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer client.Close()

		// TODO
		resources, err = apiclient.GetAllUnifiedResources(cf.Context, client.AuthClient, &proto.ListUnifiedResourcesRequest{
			Kinds:               []string{types.KindGitServer},
			SortBy:              types.SortBy{Field: types.ResourceMetadataName},
			SearchKeywords:      tc.SearchKeywords,
			PredicateExpression: tc.PredicateExpression,
			IncludeLogins:       true,
		})
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO sort
	return printGitServers(cf, resources)
}

func printGitServers(cf *CLIConf, resources []*types.EnrichedResource) error {
	format := strings.ToLower(cf.Format)
	switch format {
	case teleport.Text, "":
		return printGitServersAsText(cf, resources)
	case teleport.JSON, teleport.YAML:
		/* TODO
		out, err := serializeNodes(nodes, format)
		if err != nil {
			return trace.Wrap(err)
		}
		if _, err := fmt.Fprintln(cf.Stdout(), out); err != nil {
			return trace.Wrap(err)
		}
		*/
		return trace.BadParameter("unsupported format %q", format)
	default:
		return trace.BadParameter("unsupported format %q", format)
	}
	return nil
}

func printGitServersAsText(cf *CLIConf, resources []*types.EnrichedResource) error {
	// TODO(greedy52) verbose mode?
	profile, err := cf.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}
	var rows [][]string
	for _, resource := range resources {
		server, ok := resource.ResourceWithLabels.(types.Server)
		if !ok {
			return trace.BadParameter("expecting Git server but got %v", server)
		}

		login := "(missing username)"
		if profile.GitHubUsername != "" && profile.GitHubUserID != "" {
			login = profile.GitHubUsername
		}

		if github := server.GetGitHub(); github != nil {
			rows = append(rows, []string{
				"GitHub",
				github.Organization,
				login,
				fmt.Sprintf("https://github.com/%s", github.Organization),
			})
		} else {
			return trace.BadParameter("expecting Git server but got %v", server)
		}
	}

	t := asciitable.MakeTable([]string{"Type", "Organization", "Username", "URL"}, rows...)
	if _, err := fmt.Fprintln(cf.Stdout(), t.AsBuffer().String()); err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintln(cf.Stdout(), listGitServerHint)

	return nil
}

func parseGitURL(url string) (string, string, error) {
	// TODO support https url
	_, hostAndMore, ok := strings.Cut(url, "@")
	if !ok {
		return "", "", trace.BadParameter("cannot parse git URL %s", url)
	}
	host, orgAndMore, ok := strings.Cut(hostAndMore, ":")
	if !ok {
		return "", "", trace.BadParameter("cannot parse git URL %s", url)
	}
	org, _, ok := strings.Cut(orgAndMore, "/")
	if !ok {
		return "", "", trace.BadParameter("cannot parse git URL %s", url)
	}
	return host, org, nil
}

func makeGitSSHCommand(org string) string {
	return "tsh git ssh --github-org " + org
}

func getGitHubUserID(cf *CLIConf, tc *client.TeleportClient) (string, error) {
	profile, err := cf.ProfileStatus()
	if err != nil {
		return "", trace.Wrap(err)
	}

	if profile.GitHubUserID != "" {
		return profile.GitHubUserID, nil
	}

	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		return tc.ReissueWithGitHubAuth(cf.Context, cf.GitHubOrg)
	})
	if err != nil {
		return "", trace.Wrap(err)
	}

	profile, err = tc.ProfileStatus()
	if err != nil {
		return "", trace.Wrap(err)
	}

	if profile.GitHubUserID == "" {
		return "", trace.BadParameter("cannot retrieve github identity")
	}
	return profile.GitHubUserID, nil
}

// TODO move to prompt lib
func maybeUseTTYAsStdinFallback(ctx context.Context) {
	if prompt.Stdin().IsTerminal() {
		return
	}

	tty, err := os.Open("/dev/tty")
	if err != nil {
		log.Warn("Cannot open /dev/tty.")
		return
	}

	// Check if IsTerminal just to be safe.
	cr := prompt.NewContextReader(tty)
	if !cr.IsTerminal() {
		defer tty.Close()
		log.Warn("/dev/tty is not terminal.")
		return
	}

	go cr.HandleInterrupt()
	prompt.SetStdin(cr)

	go func() {
		<-ctx.Done()
		tty.Close()
	}()
}

const listGitServerHint = "" +
	"hint: use 'tsh git clone <git-clone-ssh-url>' to clone a new repository\n" +
	"      use 'tsh git config update' to configure an existing repository to use Teleport\n" +
	"      once the repository is cloned or configured, use 'git' as normal\n"
