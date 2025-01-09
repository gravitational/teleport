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

package configure

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/utils"
)

type ghExtraFlags struct {
	connectorName      string
	ignoreMissingRoles bool
}

func addGithubCommand(cmd *SSOConfigureCommand) *AuthKindCommand {
	spec := types.GithubConnectorSpecV3{}

	gh := &ghExtraFlags{}

	sub := cmd.ConfigureCmd.Command("github", "Configure GitHub auth connector.")
	// commonly used flags
	sub.Flag("name", "Connector name.").Default("github").Short('n').StringVar(&gh.connectorName)
	sub.Flag("teams-to-roles", "Sets teams-to-roles mapping using format 'organization,name,role1,role2,...'. Repeatable.").
		Short('r').
		Required().
		PlaceHolder("org,team,role1,role2,...").
		SetValue(newTeamsToRolesParser(&spec.TeamsToRoles))
	sub.Flag("display", "Sets the connector display name.").StringVar(&spec.Display)
	sub.Flag("id", "GitHub app client ID.").PlaceHolder("ID").Required().StringVar(&spec.ClientID)
	sub.Flag("secret", "GitHub app client secret.").Required().PlaceHolder("SECRET").StringVar(&spec.ClientSecret)
	sub.Flag("endpoint-url", "Endpoint URL for GitHub instance.").
		PlaceHolder("URL").
		Default(types.GithubURL).
		StringVar(&spec.EndpointURL)

	sub.Flag("api-endpoint-url", "API endpoint URL for GitHub instance.").
		PlaceHolder("URL").
		Default(types.GithubAPIURL).
		StringVar(&spec.APIEndpointURL)

	// auto
	sub.Flag("redirect-url", "Authorization callback URL.").PlaceHolder("URL").StringVar(&spec.RedirectURL)

	// ignores
	sub.Flag("ignore-missing-roles", "Ignore missing roles referenced in --teams-to-roles.").BoolVar(&gh.ignoreMissingRoles)

	sub.Alias("gh")

	sub.Alias(`
Examples:

  > tctl sso configure gh -r octocats,admin,access,editor,auditor -r octocats,dev,access --secret GH_SECRET --id CLIENT_ID

  Generate GitHub auth connector. Two role mappings are defined:
    - members of 'admin' team in 'octocats' org will receive 'access', 'editor' and 'auditor' roles.
    - members of 'dev' team in 'octocats' org will receive 'access' role.

  The values for --secret and --id are provided by GitHub.

  > tctl sso configure gh ... | tctl sso test

  Generate the configuration and immediately test it using "tctl sso test" command.`)

	preset := &AuthKindCommand{
		Run: func(ctx context.Context, clt *authclient.Client) error { return ghRunFunc(ctx, cmd, &spec, gh, clt) },
	}

	sub.Action(func(ctx *kingpin.ParseContext) error {
		preset.Parsed = true
		return nil
	})

	return preset
}

func ghRunFunc(ctx context.Context, cmd *SSOConfigureCommand, spec *types.GithubConnectorSpecV3, flags *ghExtraFlags, clt *authclient.Client) error {
	if err := specCheckRoles(ctx, cmd.Logger, spec, flags.ignoreMissingRoles, clt); err != nil {
		return trace.Wrap(err)
	}

	if spec.RedirectURL == "" {
		spec.RedirectURL = ResolveCallbackURL(ctx, cmd.Logger, clt, "RedirectURL", "https://%v/v1/webapi/github/callback")
	}

	connector, err := types.NewGithubConnector(flags.connectorName, *spec)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(utils.WriteYAML(os.Stdout, connector))
}

// ResolveCallbackURL deals with common pattern of resolving callback URL for IdP to use.
func ResolveCallbackURL(ctx context.Context, logger *slog.Logger, clt *authclient.Client, fieldName string, callbackPattern string) string {
	var callbackURL string

	logger.InfoContext(ctx, "resolving callback url automatically", "field_name", fieldName)
	proxies, err := clt.GetProxies()
	if err != nil {
		logger.ErrorContext(ctx, "unable to get proxy list", "error", err)
	}

	// find first proxy with public addr
	for _, proxy := range proxies {
		publicAddr := proxy.GetPublicAddr()
		if publicAddr != "" {
			callbackURL = fmt.Sprintf(callbackPattern, publicAddr)
			break
		}
	}

	// check if successfully set.
	if callbackURL == "" {
		logger.WarnContext(ctx, "Unable to resolve callback url automatically, cluster's public address unknown", "field_name", fieldName)
	} else {
		logger.InfoContext(ctx, "resolved callback url", "field_name", fieldName, "callback_url", callbackURL)
	}
	return callbackURL
}

func specCheckRoles(ctx context.Context, logger *slog.Logger, spec *types.GithubConnectorSpecV3, ignoreMissingRoles bool, clt *authclient.Client) error {
	allRoles, err := clt.GetRoles(ctx)
	if err != nil {
		logger.WarnContext(ctx, "Unable to get roles list, skipping teams-to-roles sanity checks", "error", err)
		return nil
	}

	roleMap := map[string]struct{}{}
	roleNames := make([]string, 0, len(allRoles))
	for _, role := range allRoles {
		roleMap[role.GetName()] = struct{}{}
		roleNames = append(roleNames, role.GetName())
	}

	for _, mapping := range spec.TeamsToRoles {
		for _, role := range mapping.Roles {
			_, found := roleMap[role]
			if !found {
				if ignoreMissingRoles {
					logger.WarnContext(ctx, "teams-to-roles references non-existing role",
						"non_existent_role", role,
						"available_roles", roleNames,
					)
				} else {
					return trace.BadParameter("teams-to-roles references non-existing role: %v. Correct the mapping, or add --ignore-missing-roles to ignore this error. Available roles: %v.", role, roleNames)
				}
			}
		}
	}

	return nil
}
