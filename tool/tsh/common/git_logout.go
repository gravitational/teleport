/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	gitserverv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/gitserver/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/utils"
)

type gitLogoutCommand struct {
	*kingpin.CmdClause

	gitServerName      string
	gitHubOrganization string
}

func newGitLogoutCommand(parent *kingpin.CmdClause) *gitLogoutCommand {
	cmd := &gitLogoutCommand{
		CmdClause: parent.Command("logout", "Log out of a Git server."),
	}
	cmd.Arg("git-server", "Name of the git server.").StringVar(&cmd.gitServerName)
	cmd.Flag("github-org", "GitHub organization.").StringVar(&cmd.gitHubOrganization)
	return cmd
}

func (c *gitLogoutCommand) run(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	gitServer, err := resolveGitServer(cf, tc, c.gitServerName, c.gitHubOrganization)
	if err != nil {
		return trace.Wrap(err)
	}

	github := gitServer.GetGitHub()
	if github == nil {
		return trace.BadParameter("git server %v is not a GitHub server", gitServer.GetName())
	}

	if err := tc.LogoutApp(gitServer.GetName()); err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	profile, err := cf.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := utils.RemoveFileIfExist(profile.AppLocalCAPath(tc.SiteName, gitServer.GetName())); err != nil {
		logger.WarnContext(cf.Context, "Failed to clean up local CA", "error", err)
	}

	fmt.Fprintf(cf.Stdout(), "Logged out of git server %q.\n", gitServer.GetName())

	if types.GitServerHTTPEnabled(github) {
		fmt.Fprintln(cf.Stdout(), "")
		fmt.Fprintln(cf.Stdout(), "Warning: revoking GitHub credentials will require you to run")
		fmt.Fprintf(cf.Stdout(), "\"tsh git login\" again to re-authorize with GitHub.\n")
		ok, err := promptYesNo(cf, "Revoke stored GitHub credentials? (y/N) ")
		if err != nil {
			return trace.Wrap(err)
		}
		if ok {
			if err := revokeGitCredentials(cf, tc, gitServer.GetName()); err != nil {
				return trace.Wrap(err)
			}
			fmt.Fprintln(cf.Stdout(), "GitHub credentials revoked.")
		}
	}
	return nil
}

func promptYesNo(cf *CLIConf, message string) (bool, error) {
	fmt.Fprint(cf.Stdout(), message)
	var response string
	if _, err := fmt.Fscanln(cf.Stdin(), &response); err != nil {
		return false, nil
	}
	return response == "y" || response == "Y" || response == "yes", nil
}

func revokeGitCredentials(cf *CLIConf, tc *client.TeleportClient, gitServerName string) error {
	return client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		revokeReq := &gitserverv1.RevokeGitCredentialsRequest{}
		revokeReq.SetGitServerName(gitServerName)
		_, err = clusterClient.AuthClient.GitCredentialsClient().RevokeGitCredentials(cf.Context, revokeReq)
		return trace.Wrap(err)
	})
}
