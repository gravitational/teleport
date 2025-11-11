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

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/client"
)

// gitLoginCommand implements `tsh git login`.
type gitLoginCommand struct {
	*kingpin.CmdClause

	gitHubOrganization string
	force              bool
}

func newGitLoginCommand(parent *kingpin.CmdClause) *gitLoginCommand {
	cmd := &gitLoginCommand{
		CmdClause: parent.Command("login", "Opens a browser and retrieves your login from GitHub."),
	}

	// TODO(greedy52) make "github-org" optional. Most likely there is only a
	// single Git server configured anyway so do a "list" op then use the
	// organization from that Git server. If more than one Git servers are
	// found, prompt the user to pick one.
	cmd.Flag("github-org", "GitHub organization").Required().StringVar(&cmd.gitHubOrganization)
	cmd.Flag("force", "Force a login").BoolVar(&cmd.force)
	return cmd
}

func (c *gitLoginCommand) run(cf *CLIConf) error {
	identity, err := getGitHubIdentity(cf, c.gitHubOrganization, withForceOAuthFlow(c.force))
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintf(cf.Stdout(), "Your GitHub username is %s.\n", identity.Username)
	return nil
}

func getGitHubIdentity(cf *CLIConf, org string, applyOpts ...getGitHubIdentityOption) (*client.GitHubIdentity, error) {
	opts := getGitHubIdentityOptions{
		doOAuthFlow: doGitHubOAuthFlow,
	}
	for _, applyOpt := range applyOpts {
		applyOpt(&opts)
	}

	// See if GitHub identity already present.
	profile, err := cf.ProfileStatus()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if profile.GitHubIdentity != nil && !opts.forceOAuthFlow {
		return profile.GitHubIdentity, nil
	}

	// Do GitHub OAuth flow to get GitHub identity.
	if err := opts.doOAuthFlow(cf, org); err != nil {
		return nil, trace.Wrap(err)
	}

	// Check profile again.
	profile, err = cf.ProfileStatus()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if profile.GitHubIdentity == nil {
		// This should not happen if the OAuth is successful.
		return nil, trace.NotFound("GitHub identity not found after GitHub OAuth flow")
	}
	return profile.GitHubIdentity, nil
}

type getGitHubIdentityOptions struct {
	forceOAuthFlow bool
	doOAuthFlow    func(cf *CLIConf, org string) error
}

type getGitHubIdentityOption func(*getGitHubIdentityOptions)

func withForceOAuthFlow(force bool) getGitHubIdentityOption {
	return func(o *getGitHubIdentityOptions) {
		o.forceOAuthFlow = force
	}
}

func withOAuthFlowOverride(override func(*CLIConf, string) error) getGitHubIdentityOption {
	return func(o *getGitHubIdentityOptions) {
		o.doOAuthFlow = override
	}
}

func doGitHubOAuthFlow(cf *CLIConf, org string) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	// Capture active requests before starting the OAuth flow.
	profile, err := cf.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}

	err = client.RetryWithRelogin(
		cf.Context,
		tc,
		func() error {
			return tc.ReissueWithGitHubOAuth(cf.Context, org)
		},
		client.WithAfterLoginHook(func() error {
			// Update profile if a re-login is performed.
			profile, err = cf.ProfileStatus()
			return trace.Wrap(err)
		}),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	// Ideally active requests should be handled during the OAuth flow in one
	// shot but that complicates the OAuth flow by a lot. For now, we work
	// around this by manually reissuing the request IDs after the oauth flow.
	// The oauth flow is usually only a one time login anyway so we don't expect
	// this happen often.
	if len(profile.ActiveRequests) > 0 {
		// Send to stderr in case called by `git`.
		fmt.Fprintln(cf.Stderr(), "Reissuing certificates for access requests ...")
		var emptyDropRequests []string
		if err := reissueWithRequests(cf, tc, profile.ActiveRequests, emptyDropRequests); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
