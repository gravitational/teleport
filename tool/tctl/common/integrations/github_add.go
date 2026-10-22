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

package integrations

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/prompt"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/client/sso"
	"github.com/gravitational/teleport/lib/services"
)

// githubAddCommand implements `tctl integrations github add`.
type githubAddCommand struct {
	*kingpin.CmdClause
	org     string
	appName string
	dryRun  bool
	stdout  io.Writer
	stdin   func() prompt.StdinReader

	// cleanupsOnError tracks created Teleport resources if the command fails.
	cleanupsOnError []func(context.Context)
	// createGitHubAppOverride overrides createGitHubApp in tests.
	createGitHubAppOverride func(ctx context.Context, plan *githubAddPlan) (*githubApp, error)
}

func newGitHubAddCommand(parent *kingpin.CmdClause) *githubAddCommand {
	c := &githubAddCommand{
		CmdClause: parent.Command("add", "Connect a GitHub organization for Git and the GitHub CLI over HTTPS, with a new GitHub App."),
		stdout:    os.Stdout,
		stdin:     prompt.Stdin,
	}
	c.Flag("org", "GitHub organization name.").Required().StringVar(&c.org)
	c.Flag("app-name", "Name of the GitHub App to create, which must be unique on GitHub. Defaults to the cluster name.").StringVar(&c.appName)
	c.Flag("dry-run", "Print what the command would do without making any changes.").BoolVar(&c.dryRun)
	c.Alias("create")
	return c
}

// githubAddClient is the subset of the Auth client used by `tctl integrations github add`.
type githubAddClient interface {
	services.Integrations
	Ping(ctx context.Context) (proto.PingResponse, error)
	GetCurrentUser(ctx context.Context) (types.User, error)
}

func (c *githubAddCommand) run(ctx context.Context, clt githubAddClient, gitServers services.GitServers) (err error) {
	defer c.cleanup(ctx)

	plan, err := makeGitHubAddPlan(ctx, clt, c.org, c.appName)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintln(c.stdout, plan)

	if c.dryRun {
		fmt.Fprintln(c.stdout, "This is a dry run, no changes were made.")
		fmt.Fprintln(c.stdout, "This command must be run in an interactive terminal.")
		return nil
	}
	if err := c.confirm(ctx); err != nil {
		return trace.Wrap(err)
	}

	// creates the integration and the git server before the GitHub App, so most
	// failures happen before anything is created on GitHub.
	ig, err := c.createTeleportResources(ctx, clt, gitServers, plan)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintf(c.stdout, "[1/4] Created integration and git server %q.\n", plan.name)

	fmt.Fprintln(c.stdout, "[2/4] Waiting for you to create the GitHub App in your browser...")
	app, err := c.createGitHubApp(ctx, plan)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintf(c.stdout, "[2/4] Created GitHub App %q.\n", app.Name)

	if err := c.updateGitHubIntegrationCredentials(ctx, clt, plan, ig, app); err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintf(c.stdout, "[3/4] Saved the GitHub App credentials in integration %q.\n", plan.name)

	fmt.Fprintf(c.stdout, "[4/4] Now, in your browser, install the GitHub App in %q and choose which repositories it can access.\n", plan.org)
	c.cleanupsOnError = nil
	c.printNextSteps(ctx, clt, plan, app)
	return nil
}

func (c *githubAddCommand) cleanup(ctx context.Context) {
	if len(c.cleanupsOnError) == 0 {
		return
	}
	// The command context is canceled on Ctrl+C, and Ctrl+C again exits
	// without cleanup.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), apidefaults.DefaultIOTimeout)
	defer cancel()
	fmt.Fprintln(c.stdout, "Cleaning up created resources...")
	for _, cleanup := range slices.Backward(c.cleanupsOnError) {
		cleanup(ctx)
	}
}

func (c *githubAddCommand) confirm(ctx context.Context) error {
	stdin := c.stdin()
	if !stdin.IsTerminal() {
		return trace.BadParameter("This command must be run in an interactive terminal.")
	}
	ok, err := prompt.Confirmation(ctx, c.stdout, stdin, "Proceed with GitHub App creation")
	if err != nil {
		return trace.Wrap(err)
	}
	if !ok {
		return trace.Errorf("operation canceled by user")
	}
	return nil
}

type githubAddPlan struct {
	name             string
	org              string
	proxyURL         string
	oauthCallbackURL string
	appName          string
}

func makeGitHubAddPlan(ctx context.Context, clt githubAddClient, org, appName string) (*githubAddPlan, error) {
	if err := types.ValidateGitHubOrganizationName(org); err != nil {
		return nil, trace.Wrap(err)
	}

	pong, err := clt.Ping(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	} else if pong.ProxyPublicAddr == "" {
		return nil, trace.BadParameter("the cluster has no proxy public address")
	}
	proxyURL := "https://" + pong.ProxyPublicAddr

	// GitHub App names must be unique across GitHub, so default to the cluster name.
	// https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/registering-a-github-app
	appName = cmp.Or(appName, pong.ClusterName)
	return &githubAddPlan{
		name:             "github-" + org,
		org:              org,
		proxyURL:         proxyURL,
		oauthCallbackURL: proxyURL + types.IntegrationGitHubOAuthCallbackPath,
		appName:          appName,
	}, nil
}

// String describes what the command does with the plan.
func (p *githubAddPlan) String() string {
	return fmt.Sprintf(`This command connects your GitHub organization to Teleport for Git over HTTPS:

  %[1]q

Prerequisites:
  - You are an owner of the GitHub organization.
  - You are a Teleport admin who can create integrations and git servers,
    for example with the preset editor role.
  - If the organization is on GitHub Enterprise, the Teleport cluster has a
    Teleport Enterprise license.

Steps:
  1. On your behalf, tctl creates the Teleport resources for the GitHub integration.
  2. In your browser, you create the GitHub App %[2]q,
     requesting read and write access to contents, issues and pull
     requests. You can customize its name there.
  3. On your behalf, tctl saves the GitHub App credentials in Teleport.
  4. In your browser, you install the GitHub App in %[1]q
     and choose which repositories it can access.

If a step fails, tctl deletes the Teleport resources it created.
`, p.org, p.appName)
}

func (c *githubAddCommand) createTeleportResources(ctx context.Context, clt githubAddClient, gitServers services.GitServers, plan *githubAddPlan) (types.Integration, error) {
	if err := c.createGitHubIntegration(ctx, clt, plan); err != nil {
		return nil, trace.Wrap(err)
	}

	// The integration returned by CreateIntegration cannot be used to check if
	// HTTPS protocol is supported, as older Auth Services echo back fields they
	// don't support. So poll from cache.
	ig, err := waitForGitHubIntegration(ctx, clt, plan.name)
	if err != nil {
		return nil, trace.Wrap(err)
	} else if !types.IsGitProtocolAllowed(ig.GetGitHubIntegrationSpec().AllowProtocols, types.GitProtocolHTTPS) {
		return nil, trace.NotImplemented("your Teleport cluster does not support Git over HTTPS for GitHub integrations")
	}

	if err := c.createGitHubServer(ctx, gitServers, plan); err != nil {
		return nil, trace.Wrap(err)
	}
	return ig, nil
}

func (c *githubAddCommand) createGitHubIntegration(ctx context.Context, clt githubAddClient, plan *githubAddPlan) error {
	ig, err := types.NewIntegrationGitHub(
		types.Metadata{
			Name: plan.name,
		},
		&types.GitHubIntegrationSpecV1{
			Organization:     plan.org,
			OAuthCallbackURL: plan.oauthCallbackURL,
			AllowProtocols:   []string{types.GitProtocolHTTPS},
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := setGitHubIntegrationCredentials(ig, "Iv1.placeholder", "placeholder"); err != nil {
		return trace.Wrap(err)
	}
	if _, err := clt.CreateIntegration(ctx, ig); err != nil {
		return trace.Wrap(err, "creating integration %q", plan.name)
	}

	c.cleanupsOnError = append(c.cleanupsOnError, func(ctx context.Context) {
		if err := clt.DeleteIntegration(ctx, plan.name); err != nil && !trace.IsNotFound(err) {
			fmt.Fprintf(c.stdout, "Warning: failed to delete integration %[1]q: %[2]v. Please delete it manually: tctl rm integration/%[1]s\n", plan.name, err)
		}
	})
	return nil
}

func (c *githubAddCommand) createGitHubServer(ctx context.Context, gitServers services.GitServers, plan *githubAddPlan) error {
	gitServer, err := types.NewGitHubServerWithName(plan.name, types.GitHubServerMetadata{
		Organization:   plan.org,
		Integration:    plan.name,
		AllowProtocols: []string{types.GitProtocolHTTPS},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := gitServers.CreateGitServer(ctx, gitServer); err != nil {
		return trace.Wrap(err, "creating git server %q", plan.name)
	}

	c.cleanupsOnError = append(c.cleanupsOnError, func(ctx context.Context) {
		if err := gitServers.DeleteGitServer(ctx, plan.name); err != nil && !trace.IsNotFound(err) {
			fmt.Fprintf(c.stdout, "Warning: failed to delete git server %[1]q: %[2]v. Please delete it manually: tctl rm git_server/%[1]s\n", plan.name, err)
		}
	})
	return nil
}

// createGitHubApp creates a GitHub App with the GitHub App manifest flow:
// https://docs.github.com/en/apps/sharing-github-apps/registering-a-github-app-from-a-manifest
func (c *githubAddCommand) createGitHubApp(ctx context.Context, plan *githubAddPlan) (*githubApp, error) {
	if c.createGitHubAppOverride != nil {
		return c.createGitHubAppOverride(ctx, plan)
	}

	server, err := newGitHubAppLocalServer(plan)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer func() {
		// Shutdown lets the callback finish the redirect to the installation
		// page.
		ctx, cancel := context.WithTimeout(ctx, apidefaults.DefaultIOTimeout)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			server.Close()
		}
	}()

	fmt.Fprintf(c.stdout, "      If the browser doesn't open, visit: %s\n", server.url)
	if err := sso.OpenURLInBrowser("", server.url); err != nil {
		fmt.Fprintf(c.stdout, "      Could not open the browser: %v\n", err)
	}
	app, err := server.wait(ctx)
	return app, trace.Wrap(err)
}

func (c *githubAddCommand) updateGitHubIntegrationCredentials(ctx context.Context, clt githubAddClient, plan *githubAddPlan, ig types.Integration, app *githubApp) error {
	if err := setGitHubIntegrationCredentials(ig, app.ClientID, app.ClientSecret); err != nil {
		return trace.Wrap(err)
	}
	if _, err := clt.UpdateIntegration(ctx, ig); err != nil {
		fmt.Fprintf(c.stdout, "\nThe GitHub App %q was created but the integration could not be updated with its credentials.\n", app.Name)
		fmt.Fprintf(c.stdout, "Delete it at https://github.com/organizations/%s/settings/apps/%s before retrying.\n", plan.org, app.Slug)
		return trace.Wrap(err, "updating integration %q", ig.GetName())
	}
	return nil
}

func (c *githubAddCommand) printNextSteps(ctx context.Context, clt githubAddClient, plan *githubAddPlan, app *githubApp) {
	user := "<user>"
	if current, err := clt.GetCurrentUser(ctx); err == nil {
		user = current.GetName()
	}
	fmt.Fprintf(c.stdout, `
Next steps:
  1. Review the GitHub App's permissions (read and write access to contents,
     issues and pull requests), and change them if needed:
       https://github.com/organizations/%[2]s/settings/apps/%[3]s/permissions
  2. Grant a user with the preset access role access to the organization:
       tctl users update %[1]s --set-github-orgs=%[2]s
  3. As that user, log in again so the new access takes effect:
       tsh login --force
  4. Try the GitHub CLI through Teleport, which logs in to GitHub on first use:
       tsh gh -- api /user
`, user, plan.org, app.Slug)
}

func waitForGitHubIntegration(ctx context.Context, clt githubAddClient, name string) (types.Integration, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First: 500 * time.Millisecond,
		Step:  500 * time.Millisecond,
		Max:   2 * time.Second,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var ig types.Integration
	if err := retry.For(ctx, func() error {
		ig, err = clt.GetIntegration(ctx, name)
		switch {
		case err == nil:
			return nil
		case trace.IsNotFound(err):
			return trace.Wrap(err)
		default:
			return retryutils.PermanentRetryError(err)
		}
	}); err != nil {
		return nil, trace.Wrap(err, "waiting for integration %q to be created", name)
	}
	return ig, nil
}

func setGitHubIntegrationCredentials(ig types.Integration, clientID, clientSecret string) error {
	return trace.Wrap(ig.SetCredentials(&types.PluginCredentialsV1{
		Credentials: &types.PluginCredentialsV1_IdSecret{
			IdSecret: &types.PluginIdSecretCredential{
				Id:     clientID,
				Secret: clientSecret,
			},
		},
	}))
}
