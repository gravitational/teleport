// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package configure

import (
	"context"
	"fmt"
	"os"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"
)

type ghExtraFlags struct {
	connectorName      string
	ignoreMissingRoles bool
}

func addGithubCommand(cmd *SSOConfigureCommand) *AuthKindCommand {
	spec := types.GithubConnectorSpecV3{}

	gh := &ghExtraFlags{}

	sub := cmd.ConfigureCmd.Command("github", "Configure Github auth connector.")
	// commonly used flags
	sub.Flag("name", "Connector name.").Default("github").Short('n').StringVar(&gh.connectorName)
	sub.Flag("teams-to-logins", "Sets teams-to-logins mapping in the form 'organization,team_name,role1,role2,...'. Repeatable.").
		Short('r').
		Required().
		PlaceHolder("org,team,role1,role2,...").
		SetValue(newTeamsToLoginsParser(&spec.TeamsToLogins))
	sub.Flag("display", "Display controls how this connector is displayed.").StringVar(&spec.Display)
	sub.Flag("id", "Github app client ID.").PlaceHolder("ID").Required().StringVar(&spec.ClientID)
	sub.Flag("secret", "Github app client secret.").Required().PlaceHolder("SECRET").StringVar(&spec.ClientSecret)

	// auto
	sub.Flag("redirect-url", "Authorization callback URL.").PlaceHolder("URL").StringVar(&spec.RedirectURL)

	// ignores
	ignoreMissingRoles := false
	sub.Flag("ignore-missing-roles", "Ignore non-existing roles referenced in --claims-to-roles.").BoolVar(&ignoreMissingRoles)

	sub.Alias("gh")

	sub.Alias(`
Examples:

  > tctl sso configure gh -r octocats,admin,access,editor,auditor -r octocats,dev,access --secret GH_SECRET --id CLIENT_ID

  Generate Github auth connector. Two role mappings are defined:
    - members of 'admin' team in 'octocats' org will receive 'access', 'editor' and 'auditor' roles.
    - members of 'dev' team in 'octocats' org will receive 'access' role.

  The values for --secret and --id are provided by GitHub.

  > tctl sso configure gh ... | tctl sso test
  
  Generate the configuration and immediately test it using "tctl sso test" command.`)

	preset := &AuthKindCommand{
		Run: func(clt auth.ClientI) error { return ghRunFunc(cmd, &spec, gh, clt) },
	}

	sub.Action(func(ctx *kingpin.ParseContext) error {
		preset.Parsed = true
		return nil
	})

	return preset
}

func ghRunFunc(cmd *SSOConfigureCommand, spec *types.GithubConnectorSpecV3, flags *ghExtraFlags, clt auth.ClientI) error {
	allRoles, err := clt.GetRoles(context.TODO())
	if err != nil {
		cmd.Logger.WithError(err).Warn("unable to get roles list. Skipping attributes_to_roles sanity checks.")
	} else {
		roleMap := map[string]bool{}
		var roleNames []string
		for _, role := range allRoles {
			roleMap[role.GetName()] = true
			roleNames = append(roleNames, role.GetName())
		}

		for _, mapping := range spec.TeamsToLogins {
			for _, role := range mapping.Logins {
				_, found := roleMap[role]
				if !found {
					if flags.ignoreMissingRoles {
						cmd.Logger.Warnf("teams-to-logins references non-existing role: %q. Available roles: %v.", role, roleNames)
					} else {
						return trace.BadParameter("teams-to-logins references non-existing role: %v. Correct the mapping, or add --ignore-missing-roles to ignore this error. Available roles: %v.", role, roleNames)
					}
				}
			}
		}
	}

	if spec.RedirectURL == "" {
		cmd.Logger.Info("RedirectURL empty, resolving automatically.")
		proxies, err := clt.GetProxies()
		if err != nil {
			cmd.Logger.WithError(err).Error("unable to get proxy list.")
		}

		// find first proxy with public addr
		for _, proxy := range proxies {
			publicAddr := proxy.GetPublicAddr()
			if publicAddr != "" {
				// TODO: double check this is correct
				spec.RedirectURL = fmt.Sprintf("https://%v/v1/webapi/github/callback", publicAddr)
				break
			}
		}

		// check if successfully set.
		if spec.RedirectURL == "" {
			cmd.Logger.Warn("Unable to fill RedirectURL automatically, cluster's public address unknown.")
		} else {
			cmd.Logger.Infof("RedirectURL set to %q", spec.RedirectURL)
		}
	}

	connector, err := types.NewGithubConnector(flags.connectorName, *spec)

	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(utils.WriteYAML(os.Stdout, connector))
}
