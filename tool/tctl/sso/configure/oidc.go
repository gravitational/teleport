// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package configure

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"github.com/zitadel/oidc/v3/pkg/client"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/tctl/sso/configure/flags"
	"github.com/gravitational/teleport/tool/tctl/sso/tester"
)

type oidcPreset struct {
	name        string
	description string
	display     string
	issuerURL   string
	modifySpec  func(ctx context.Context, logger *slog.Logger, spec *types.OIDCConnectorSpecV3) error
}

type oidcPresetList []oidcPreset

func (lst oidcPresetList) getNames() []string {
	var names []string
	for _, p := range lst {
		names = append(names, p.name)
	}
	return names
}

func (lst oidcPresetList) getPreset(name string) *oidcPreset {
	for _, p := range lst {
		if p.name == name {
			return &p
		}
	}
	return nil
}

const presetGoogle = "google"

var oidcPresets = oidcPresetList([]oidcPreset{
	{
		name:        presetGoogle,
		description: "Google Workspace",
		display:     "Google",
		issuerURL:   "https://accounts.google.com",
		modifySpec: func(ctx context.Context, logger *slog.Logger, spec *types.OIDCConnectorSpecV3) error {
			if !strings.HasSuffix(spec.ClientID, ".apps.googleusercontent.com") {
				return trace.BadParameter(`For Google Workspace the client ID have to use format "<GOOGLE_WORKSPACE_CLIENT_ID>.apps.googleusercontent.com", got %q instead. Set full value with --id=... or shorthand with --google-id=...`, spec.ClientID)
			}

			if spec.GoogleServiceAccount == "" && spec.GoogleServiceAccountURI == "" {
				return trace.BadParameter("Provide Google service account credentials file with --google-acc-uri=file://<path> or --google-acc=...")
			}

			if spec.GoogleAdminEmail == "" {
				return trace.BadParameter("Provide Google admin email address with --google-admin=...")
			}

			return nil
		},
	},

	{
		name:        "gitlab",
		description: "GitLab",
		display:     "GitLab",
		issuerURL:   "https://gitlab.com",
		modifySpec: func(ctx context.Context, logger *slog.Logger, spec *types.OIDCConnectorSpecV3) error {
			switch spec.Prompt {
			case "none":
				break
			case "":
				spec.Prompt = "none"
			default:
				logger.WarnContext(ctx, "GitLab 'prompt' parameter was not set to required value of 'none'", "prompt", spec.Prompt)
			}

			return nil
		},
	},

	{
		name:        "okta",
		description: "Okta",
		display:     "Okta",
		issuerURL:   "https://oktaice.okta.com",
		modifySpec: func(ctx context.Context, logger *slog.Logger, spec *types.OIDCConnectorSpecV3) error {
			if spec.Provider == "" {
				spec.Provider = teleport.Okta
			}

			if spec.Provider != teleport.Okta {
				logger.WarnContext(ctx, "Configured provider was not okta", "provider", spec.Provider)
			}

			return nil
		},
	},
})

type oidcExtraFlags struct {
	chosenPreset       string
	connectorName      string
	googleID           string
	googleLegacy       bool
	ignoreMissingRoles bool
}

func addOIDCCommand(cmd *SSOConfigureCommand) *AuthKindCommand {
	spec := types.OIDCConnectorSpecV3{}

	pTable := asciitable.MakeTable([]string{"Name", "Description", "Display", "Issuer URL"})
	for _, preset := range oidcPresets {
		pTable.AddRow([]string{preset.name, preset.description, preset.display, preset.issuerURL})
	}
	presets := tester.Indent(pTable.AsBuffer().String(), 2)

	extra := &oidcExtraFlags{}

	sub := cmd.ConfigureCmd.Command("oidc", fmt.Sprintf("Configure OIDC auth connector, optionally using a preset. Available presets: %v.", oidcPresets.getNames()))
	// commonly used flags
	sub.Flag("preset", fmt.Sprintf("Preset. One of: %v", oidcPresets.getNames())).Short('p').EnumVar(&extra.chosenPreset, oidcPresets.getNames()...)
	sub.Flag("name", "Connector name. Required, unless implied from preset.").Short('n').StringVar(&extra.connectorName)
	sub.Flag("claims-to-roles", "Sets claim-to-roles mapping using format 'claim_name,claim_value,role1,role2,...'. Repeatable.").Short('r').Required().PlaceHolder("name,value,role1,role2,...").SetValue(flags.NewClaimsToRolesParser(&spec.ClaimsToRoles))
	sub.Flag("display", "Sets the connector display name.").StringVar(&spec.Display)
	sub.Flag("id", "OIDC app client ID.").PlaceHolder("ID").StringVar(&spec.ClientID)
	sub.Flag("secret", "OIDC app client secret.").Required().PlaceHolder("SECRET").StringVar(&spec.ClientSecret)
	sub.Flag("issuer-url", "Issuer URL.").PlaceHolder("URL").StringVar(&spec.IssuerURL)

	// auto
	sub.Flag("redirect-url", "Authorization callback URL(s). Each repetition of the flag declares one redirectURL.").PlaceHolder("https://<proxy>/v1/webapi/oidc/callback").StringsVar((*[]string)(&spec.RedirectURLs))

	// rarely used
	sub.Flag("prompt", "Optional OIDC prompt. Example values: none, select_account, login, consent.").StringVar(&spec.Prompt)
	sub.Flag("scope", "Scope specifies additional scopes set by provider. Each repetition of the flag declares one scope. Examples: email, groups, openid.").StringsVar(&spec.Scope)
	sub.Flag("acr", "Authentication Context Class Reference values.").StringVar(&spec.ACR)
	sub.Flag("provider", "Sets the external identity provider type to enable IdP specific workarounds. Examples: ping, adfs, netiq, okta.").StringVar(&spec.Provider)

	// google only.
	sub.Flag("google-acc-uri", "Google only. URI pointing at service account credentials. Example: file:///var/lib/teleport/gworkspace-creds.json.").StringVar(&spec.GoogleServiceAccountURI)
	sub.Flag("google-acc", "Google only. String containing Google service account credentials.").StringVar(&spec.GoogleServiceAccount)
	sub.Flag("google-admin", "Google only. Email of a Google admin to impersonate.").StringVar(&spec.GoogleAdminEmail)
	sub.Flag("google-legacy", "Google only. Flag to select groups with direct membership filtered by domain (legacy behavior). Disabled by default. More info: https://goteleport.com/docs/enterprise/sso/google-workspace/#how-teleport-uses-google-workspace-apis").BoolVar(&extra.googleLegacy)

	sub.Flag("google-id", "Shorthand for setting the --id flag to <GOOGLE_WORKSPACE_CLIENT_ID>.apps.googleusercontent.com").PlaceHolder("GOOGLE_WORKSPACE_CLIENT_ID").StringVar(&extra.googleID)

	// ignores
	ignoreMissingRoles := false
	sub.Flag("ignore-missing-roles", "Ignore missing roles referenced in --claims-to-roles.").BoolVar(&ignoreMissingRoles)

	sub.Alias(fmt.Sprintf(`
Presets:

%v
Examples:

  > tctl sso configure oidc -n myauth -r groups,admin,access,editor,auditor -r group,developer,access --secret IDP_SECRET --id CLIENT_ID --issuer-url https://idp.example.com

  Generate OIDC auth connector configuration called 'myauth'. Two mappings from OIDC claims to roles are defined:
    - members of 'admin' group will receive 'access', 'editor' and 'auditor' roles.
    - members of 'developer' group will receive 'access' role.

  The values for --secret, --id and --issuer-url are provided by IdP.

  > tctl sso configure oidc --preset okta --scope groups -r groups,okta-admin,access,editor,auditor --secret IDP_SECRET --id CLIENT_ID --issuer-url dev-123456.oktapreview.com

  Generate OIDC auth connector with Okta preset, enabled 'groups' scope, mapping group 'okta-admin' to roles 'access', 'editor', 'auditor'.
  Issuer URL is set to match custom Okta domain.

  > tctl sso configure oidc --preset google -r groups,mygroup@mydomain.example.com,access --secret SECRET --google-id GOOGLE_ID --google-acc-uri /var/lib/teleport/gacc.json --google-admin admin@mydomain.example.com

  Generate OIDC auth connector with Google preset. Service account credentials are set to be loaded from /var/lib/teleport/gacc.json with --google-acc-uri.

  > tctl sso configure oidc ... | tctl sso test

  Generate the configuration and immediately test it using "tctl sso test" command.`, presets))

	preset := &AuthKindCommand{
		Run: func(ctx context.Context, clt *authclient.Client) error {
			return oidcRunFunc(ctx, cmd, &spec, extra, clt)
		},
	}

	sub.Action(func(ctx *kingpin.ParseContext) error {
		preset.Parsed = true
		return nil
	})

	return preset
}

func oidcRunFunc(ctx context.Context, cmd *SSOConfigureCommand, spec *types.OIDCConnectorSpecV3, flags *oidcExtraFlags, clt *authclient.Client) error {
	if flags.googleID != "" {
		if spec.ClientID != "" {
			return trace.BadParameter("Conflicting flags: --id and --google-id. Provide only one.")
		}
		spec.ClientID = flags.googleID + ".apps.googleusercontent.com"
	}

	if spec.GoogleServiceAccountURI != "" {
		_, err := apiutils.ParseSessionsURI(spec.GoogleServiceAccountURI)
		if err != nil {
			return trace.BadParameter("Failed to parse --google-acc-uri: %v", err)
		}
	}

	// automatically switch to 'google' preset if google-specific flags are set.
	if flags.chosenPreset == "" {
		if spec.GoogleAdminEmail != "" || spec.GoogleServiceAccount != "" || spec.GoogleServiceAccountURI != "" {
			cmd.Logger.InfoContext(ctx, "Google-specific flags detected, enabling google preset")
			flags.chosenPreset = presetGoogle
		}
	}

	// apply preset, if chosen
	p := oidcPresets.getPreset(flags.chosenPreset)
	if p != nil {
		if spec.Display == "" {
			spec.Display = p.display
		}

		if spec.IssuerURL == "" {
			spec.IssuerURL = p.issuerURL
		}

		if flags.connectorName == "" {
			flags.connectorName = p.name
		}

		if p.modifySpec != nil {
			if err := p.modifySpec(ctx, cmd.Logger, spec); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	// check ClientID *after* applying presets. This is because 'google' preset will validate the value of spec.ClientID.
	if spec.ClientID == "" {
		return trace.BadParameter("Missing client ID. Provide with --id.")
	}

	if spec.IssuerURL == "" {
		return trace.BadParameter("Missing issuer URL. Provide with --issuer-url or choose a preset with one.")
	}
	parse, err := url.Parse(spec.IssuerURL)
	if err != nil {
		return trace.Wrap(err, "Invalid issuer URL.")
	}

	switch strings.ToLower(parse.Scheme) {
	case "":
		spec.IssuerURL = "https://" + spec.IssuerURL
		cmd.Logger.InfoContext(ctx, "Missing scheme for issuer URL, using https", "issuer_url", spec.IssuerURL)
	case "https":
		break
	default:
		return trace.BadParameter("Bad scheme for issuer URL %q. Expected %q got %q.", spec.IssuerURL, "https", parse.Scheme)
	}

	// verify .well-known/openid-configuration is reachable
	if _, err := client.Discover(ctx, spec.IssuerURL, otelhttp.DefaultClient); err != nil {
		if cmd.Config.Debug {
			cmd.Logger.WarnContext(ctx, "Failed to load .well-known/openid-configuration for issuer URL", "issuer_url", spec.IssuerURL, "error", err)
		}
		return trace.BadParameter("Failed to load .well-known/openid-configuration for issuer URL %q. Check expected --issuer-url against IdP configuration. Rerun with --debug to see the error.", spec.IssuerURL)
	}

	if flags.connectorName == "" {
		return trace.BadParameter("Connector name must be set, either by choosing --preset or explicitly via --name")
	}

	allRoles, err := clt.GetRoles(ctx)
	if err != nil {
		cmd.Logger.WarnContext(ctx, "unable to get roles list, skipping attributes_to_roles sanity checks", "error", err)
	} else {
		roleMap := map[string]bool{}
		var roleNames []string
		for _, role := range allRoles {
			roleMap[role.GetName()] = true
			roleNames = append(roleNames, role.GetName())
		}

		for _, mapping := range spec.ClaimsToRoles {
			for _, role := range mapping.Roles {
				_, found := roleMap[role]
				if !found {
					if flags.ignoreMissingRoles {
						cmd.Logger.WarnContext(ctx, "claims-to-roles references non-existing role", "role", role, "available_roles", roleNames)
					} else {
						return trace.BadParameter("claims-to-roles references non-existing role: %v. Correct the mapping, or add --ignore-missing-roles to ignore this error. Available roles: %v.", role, roleNames)
					}
				}
			}
		}
	}

	if len(spec.RedirectURLs) == 0 {
		spec.RedirectURLs = []string{ResolveCallbackURL(ctx, cmd.Logger, clt, "RedirectURLs", "https://%v/v1/webapi/oidc/callback")}
	}

	connector, err := types.NewOIDCConnector(flags.connectorName, *spec)
	if err != nil {
		return trace.Wrap(err)
	}

	if flags.googleLegacy {
		// The Google Workspace groups fetcher follows the legacy behavior when the version of the OIDCConnector is v2.
		if cv3, ok := connector.(*types.OIDCConnectorV3); ok {
			cv3.Version = types.V2
		} else {
			return trace.BadParameter("Unable to set connector version to %q, bad type: %v", types.V2, connector)
		}
	}

	return trace.Wrap(utils.WriteYAML(os.Stdout, connector))
}
