// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package plugin

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	apicommon "github.com/gravitational/teleport/api/types/common"
	icfilters "github.com/gravitational/teleport/lib/aws/identitycenter/filters"
	"github.com/gravitational/teleport/lib/client"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
	icutils "github.com/gravitational/teleport/lib/utils/aws/identitycenterutils"
	awsregion "github.com/gravitational/teleport/lib/utils/aws/region"
)

const (
	defaultAWSICPluginName = apicommon.OriginAWSIdentityCenter
	awsicPluginNameFlag    = "plugin-name"
	awsicPluginNameHelp    = "Name of the AWS Identity Center integration instance to update. Defaults to " + apicommon.OriginAWSIdentityCenter + "."
	awsicRolesSyncModeFlag = "roles-sync-mode"
	awsicRolesSyncModeHelp = "Control account-assignment role creation. ALL creates roles for all possible account assignments. NONE creates no roles, and also implies a totally-exclusive group import filter."
	notAWSICPluginMsg      = "%q is not an AWS Identity Center integration"
)

type awsICInstallArgs struct {
	cmd                       *kingpin.CmdClause
	defaultOwners             []string
	scimToken                 string
	scimURL                   *url.URL
	forceSCIMURL              bool
	region                    string
	arn                       string
	useSystemCredentials      bool
	assumeRoleARN             string
	userOrigins               []string
	userLabels                []string
	groupNameFilters          []string
	accountNameFilters        []string
	accountIDFilters          []string
	rolesSyncMode             string
	excludeGroupNameFilters   []string
	excludeAccountNameFilters []string
	excludeAccountIDFilters   []string
}

func (a *awsICInstallArgs) validate(ctx context.Context, log *slog.Logger) error {
	if !awsregion.IsKnownRegion(a.region) {
		return trace.BadParameter("unknown AWS region: %s", a.region)
	}

	if a.scimToken == "" {
		return trace.BadParameter("SCIM token must not be empty")
	}

	if err := a.validateSystemCredentialInput(); err != nil {
		return trace.Wrap(err)
	}

	if err := a.validateSCIMBaseURL(ctx, log); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (a *awsICInstallArgs) validateSystemCredentialInput() error {
	if !a.useSystemCredentials {
		return trace.BadParameter("--use-system-credentials must be set. The tctl-based AWS IAM Identity Center plugin installation only supports AWS local system credentials")
	}

	if a.assumeRoleARN == "" {
		return trace.BadParameter("--assume-role-arn must be set when --use-system-credentials is configured")
	}

	if _, err := awsutils.ParseRoleARN(a.assumeRoleARN); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (a *awsICInstallArgs) validateSCIMBaseURL(ctx context.Context, log *slog.Logger) error {
	validatedBaseUrl, err := icutils.EnsureSCIMEndpointURL(a.scimURL)
	if err == nil {
		a.scimURL = validatedBaseUrl
		return nil

	}

	if a.forceSCIMURL {
		log.WarnContext(ctx, "Using supplied SCIM base URL even though it is not a valid AWS IC SCIM base URL.")
		return nil
	}

	return trace.Wrap(err)
}

func (a *awsICInstallArgs) parseGroupFilters() (icfilters.Filters, error) {
	filters := make([]*types.AWSICResourceFilter, 0, len(a.groupNameFilters)+len(a.excludeGroupNameFilters))
	for _, n := range a.groupNameFilters {
		filters = append(filters, &types.AWSICResourceFilter{
			Include: &types.AWSICResourceFilter_NameRegex{NameRegex: n},
		})
	}
	for _, n := range a.excludeGroupNameFilters {
		filters = append(filters, &types.AWSICResourceFilter{
			Exclude: &types.AWSICResourceFilter_ExcludeNameRegex{ExcludeNameRegex: n},
		})
	}
	return icfilters.New(filters)
}

func (a *awsICInstallArgs) parseAccountFilters() (icfilters.Filters, error) {
	filtersCap := len(a.accountNameFilters) + len(a.excludeAccountNameFilters) + len(a.accountIDFilters) + len(a.excludeAccountIDFilters)
	filters := make([]*types.AWSICResourceFilter, 0, filtersCap)
	for _, n := range a.accountNameFilters {
		filters = append(filters, &types.AWSICResourceFilter{
			Include: &types.AWSICResourceFilter_NameRegex{NameRegex: n},
		})
	}

	for _, id := range a.accountIDFilters {
		filters = append(filters, &types.AWSICResourceFilter{
			Include: &types.AWSICResourceFilter_Id{Id: id},
		})
	}

	for _, n := range a.excludeAccountNameFilters {
		filters = append(filters, &types.AWSICResourceFilter{
			Exclude: &types.AWSICResourceFilter_ExcludeNameRegex{ExcludeNameRegex: n},
		})
	}

	for _, id := range a.excludeAccountIDFilters {
		filters = append(filters, &types.AWSICResourceFilter{
			Exclude: &types.AWSICResourceFilter_ExcludeId{ExcludeId: id},
		})
	}

	return icfilters.New(filters)
}

func (a *awsICInstallArgs) parseUserFilters() ([]*types.AWSICUserSyncFilter, error) {
	result := make([]*types.AWSICUserSyncFilter, 0, len(a.userOrigins)+len(a.userLabels))

	if len(a.userOrigins) > 0 {
		result = make([]*types.AWSICUserSyncFilter, 0, len(a.userOrigins))
		for _, origin := range a.userOrigins {
			result = append(result, &types.AWSICUserSyncFilter{
				Labels: map[string]string{types.OriginLabel: origin},
			})
		}
	}

	if len(a.userLabels) > 0 {
		result = slices.Grow(result, len(a.userLabels))
		for _, labelSpec := range a.userLabels {
			labels, err := client.ParseLabelSpec(labelSpec)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			result = append(result, &types.AWSICUserSyncFilter{Labels: labels})
		}
	}

	return result, nil
}

func (p *PluginsCommand) initInstallAWSIC(parent *kingpin.CmdClause) {
	p.install.awsIC.cmd = parent.Command("awsic", "Install an AWS IAM Identity Center integration.")
	cmd := p.install.awsIC.cmd
	cmd.Flag("access-list-default-owner", "Teleport user to set as default owner for the imported access lists. Multiple flags allowed.").
		Required().
		StringsVar(&p.install.awsIC.defaultOwners)
	cmd.Flag("scim-url", "AWS Identity Center SCIM provisioning endpoint").
		Required().
		URLVar(&p.install.awsIC.scimURL)
	cmd.Flag("force-scim-url", "Use the provided SCIM provisioning endpoint even if it fails scim endpoint validation").
		Default("false").
		BoolVar(&p.install.awsIC.forceSCIMURL)
	cmd.Flag("scim-token", "AWS Identify Center SCIM provisioning token.").
		Required().
		StringVar(&p.install.awsIC.scimToken)
	cmd.Flag("instance-region", "AWS Identity Center instance region").
		Required().
		StringVar(&p.install.awsIC.region)
	cmd.Flag("instance-arn", "AWS Identity center instance ARN").
		Required().
		StringVar(&p.install.awsIC.arn)
	cmd.Flag("use-system-credentials", "Uses system credentials instead of OIDC.").
		Default("true").
		BoolVar(&p.install.awsIC.useSystemCredentials)
	cmd.Flag("assume-role-arn", "ARN of a role that the system credential should assume.").
		StringVar(&p.install.awsIC.assumeRoleARN)

	cmd.Flag("user-origin", fmt.Sprintf(`Shorthand for "--user-label %s=ORIGIN"`, types.OriginLabel)).
		PlaceHolder("ORIGIN").
		EnumsVar(&p.install.awsIC.userOrigins, types.OriginValues...)
	cmd.Flag("user-label", "Add user label filter, in the form of a comma-separated list of \"name=value\" pairs. If no label filters are supplied, all Teleport users will be provisioned to Identity Center").
		PlaceHolder("LABELSPEC").
		StringsVar(&p.install.awsIC.userLabels)

	cmd.Flag("group-name", "Add AWS group to group import list by name. Can be a glob, or enclosed in ^$ to specify a regular expression. If no filters are supplied then all AWS groups will be imported.").
		StringsVar(&p.install.awsIC.groupNameFilters)
	cmd.Flag("account-name", "Add AWS Account to account import list by name. Can be a glob, or enclosed in ^$ to specify a regular expression. All AWS accounts will be imported if no items are added to account import list.").
		StringsVar(&p.install.awsIC.accountNameFilters)
	cmd.Flag("account-id", "Add AWS Account to account import list by ID. All AWS accounts will be imported if no items are added to account import list.").
		StringsVar(&p.install.awsIC.accountIDFilters)

	cmd.Flag("exclude-group-name", "Exclude AWS group from import list by name. Can be a glob or a regular expression (enclosed in ^$).").
		StringsVar(&p.install.awsIC.excludeGroupNameFilters)
	cmd.Flag("exclude-account-name", "Exclude AWS account from import list by name. Can be a glob or a regular expression (enclosed in ^$).").
		StringsVar(&p.install.awsIC.excludeAccountNameFilters)
	cmd.Flag("exclude-account-id", "Exclude AWS account from import list by ID.").
		StringsVar(&p.install.awsIC.excludeAccountIDFilters)

	cmd.Flag("roles-sync-mode", "Control account-assignment role creation. ALL creates Teleport Roles for all possible account assignments. NONE creates no Teleport Roles, and also implies a totally-exclusive group import filter.").
		Default(types.AWSICRolesSyncModeAll).
		EnumVar(&p.install.awsIC.rolesSyncMode, types.AWSICRolesSyncModeAll, types.AWSICRolesSyncModeNone)
}

// InstallAWSIC installs AWS Identity Center plugin.
func (p *PluginsCommand) InstallAWSIC(ctx context.Context, args pluginServices) error {
	awsICArgs := p.install.awsIC
	if err := awsICArgs.validate(ctx, p.config.Logger); err != nil {
		return trace.Wrap(err)
	}

	userFilters, err := awsICArgs.parseUserFilters()
	if err != nil {
		return trace.Wrap(err)
	}

	groupFilters, err := awsICArgs.parseGroupFilters()
	if err != nil {
		return trace.Wrap(err)
	}

	if awsICArgs.rolesSyncMode == types.AWSICRolesSyncModeNone {
		if len(groupFilters) != 0 {
			return trace.BadParameter("specifying group import filers is incompatible with --roles-sync-mode NONE")
		}
		groupFilters = append(groupFilters, &types.AWSICResourceFilter{
			Exclude: &types.AWSICResourceFilter_ExcludeNameRegex{ExcludeNameRegex: "*"},
		})
	}

	accountFilters, err := awsICArgs.parseAccountFilters()
	if err != nil {
		return trace.Wrap(err)
	}

	settings := &types.PluginAWSICSettings{
		Region: awsICArgs.region,
		Arn:    awsICArgs.arn,
		ProvisioningSpec: &types.AWSICProvisioningSpec{
			BaseUrl: awsICArgs.scimURL.String(),
		},
		AccessListDefaultOwners: awsICArgs.defaultOwners,
		UserSyncFilters:         userFilters,
		GroupSyncFilters:        groupFilters,
		AwsAccountsFilters:      accountFilters,
		RolesSyncMode:           awsICArgs.rolesSyncMode,
	}

	if awsICArgs.useSystemCredentials {
		settings.Credentials = &types.AWSICCredentials{
			Source: &types.AWSICCredentials_System{
				System: &types.AWSICCredentialSourceSystem{
					AssumeRoleArn: awsICArgs.assumeRoleARN,
				},
			},
		}

		// Set the deprecated CredentialsSource to the legacy value to allow old
		// versions of Teleport to handle the record. DELETE in Teleport 19
		settings.CredentialsSource = types.AWSICCredentialsSource_AWSIC_CREDENTIALS_SOURCE_SYSTEM
	}

	req := &pluginspb.CreatePluginRequest{
		Plugin: &types.PluginV1{
			Metadata: types.Metadata{
				Name: apicommon.OriginAWSIdentityCenter,
				Labels: map[string]string{
					"teleport.dev/hosted-plugin": "true",
				},
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_AwsIc{
					AwsIc: settings,
				},
			},
		},
		StaticCredentials: &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Labels: map[string]string{
						"aws-ic/scim-api-endpoint": awsICArgs.scimURL.String(),
					},
					Name: types.PluginTypeAWSIdentityCenter,
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
					APIToken: awsICArgs.scimToken,
				},
			},
		},
	}

	_, err = args.plugins.CreatePlugin(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("Successfully created AWS Identity Center plugin.")

	return nil
}

type awsICEditArgs struct {
	cmd           *kingpin.CmdClause
	pluginName    string
	rolesSyncMode string
}

func (p *PluginsCommand) initEditAWSIC(parent *kingpin.CmdClause) {
	p.edit.awsIC.cmd = parent.Command("awsic", "Edit an AWS IAM Identity Center integration's settings.")
	cmd := p.edit.awsIC.cmd

	cmd.Flag(awsicPluginNameFlag, awsicPluginNameHelp).
		Default(defaultAWSICPluginName).
		StringVar(&p.edit.awsIC.pluginName)

	cmd.Flag(awsicRolesSyncModeFlag, awsicRolesSyncModeHelp).
		EnumVar(&p.edit.awsIC.rolesSyncMode, types.AWSICRolesSyncModeAll, types.AWSICRolesSyncModeNone)
}

func (p *PluginsCommand) EditAWSIC(ctx context.Context, args pluginServices) error {
	plugin, err := args.plugins.GetPlugin(ctx, &pluginspb.GetPluginRequest{
		Name: p.edit.awsIC.pluginName,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	icSettings := plugin.Spec.GetAwsIc()
	if icSettings == nil {
		return trace.BadParameter("%q is not an AWS Identity Center integration", p.edit.awsIC.pluginName)
	}

	cliArgs := &p.edit.awsIC
	if cliArgs.rolesSyncMode != "" {
		icSettings.RolesSyncMode = cliArgs.rolesSyncMode
	}

	_, err = args.plugins.UpdatePlugin(ctx, &pluginspb.UpdatePluginRequest{
		Plugin: plugin,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

type awsICRotateCredsArgs struct {
	cmd               *kingpin.CmdClause
	pluginName        string
	payload           string
	requireValidation bool
}

func (p *PluginsCommand) initRotateCredsAWSIC(parent *kingpin.CmdClause) {
	p.rotateCreds.awsic.cmd = parent.Command("awsic", "Rotate the AWS Identity Center SCIM bearer token.")
	cmd := p.rotateCreds.awsic.cmd
	args := &p.rotateCreds.awsic

	cmd.Flag("plugin-name", "Name of the AWSIC plugin instance to update. Defaults to "+apicommon.OriginAWSIdentityCenter+".").
		Default(apicommon.OriginAWSIdentityCenter).
		StringVar(&args.pluginName)

	cmd.Arg("token", "The new SCIM bearer token.").
		PlaceHolder("TOKEN").
		Required().
		StringVar(&p.rotateCreds.awsic.payload)

	cmd.Flag("validate-token", "Validate that the supplied token is valid for the configured downstream SCIM service").
		Default("true").
		BoolVar(&args.requireValidation)
}

func (p *PluginsCommand) RotateAWSICCreds(ctx context.Context, args pluginServices) error {
	cliArgs := &p.rotateCreds.awsic

	slog.InfoContext(ctx, "Fetching plugin...", "plugin_name", cliArgs.pluginName)
	plugin, err := args.plugins.GetPlugin(ctx, &pluginspb.GetPluginRequest{
		Name:        cliArgs.pluginName,
		WithSecrets: true,
	})
	if err != nil {
		return trace.Wrap(err, "fetching plugin %q", cliArgs.pluginName)
	}

	awsicSettings := plugin.Spec.GetAwsIc()
	if awsicSettings == nil {
		return trace.BadParameter(notAWSICPluginMsg, cliArgs.pluginName)
	}

	if p.rotateCreds.awsic.requireValidation {
		if err := p.rotateCreds.awsic.validateToken(ctx, awsicSettings, args); err != nil {
			return trace.Wrap(err, "validating SCIM bearer token")
		}
	}

	staticCredsRef := plugin.Credentials.GetStaticCredentialsRef()
	if staticCredsRef == nil {
		return trace.BadParameter("plugin has no credentials reference")
	}

	req := pluginspb.UpdatePluginStaticCredentialsRequest{
		Target: &pluginspb.UpdatePluginStaticCredentialsRequest_Query{
			Query: &pluginspb.CredentialQuery{
				Labels: staticCredsRef.Labels,
			},
		},
		Credential: &types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
				APIToken: p.rotateCreds.awsic.payload,
			},
		},
	}

	_, err = args.plugins.UpdatePluginStaticCredentials(ctx, &req)
	if err != nil {
		return trace.Wrap(err, "updating credentials")
	}

	return nil
}

func (args *awsICRotateCredsArgs) validateToken(ctx context.Context, awsicSettings *types.PluginAWSICSettings, env pluginServices) error {
	provisioningSpec := awsicSettings.ProvisioningSpec
	if provisioningSpec == nil {
		return trace.BadParameter("plugin is missing provisioning spec")
	}

	slog.InfoContext(ctx, "Validating token", "scim_server", provisioningSpec.BaseUrl)

	endPoint, err := url.JoinPath(provisioningSpec.BaseUrl, "ServiceProviderConfig")
	if err != nil {
		return trace.Wrap(err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endPoint, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", args.payload))
	req.Header.Set("Accept", "application/scim+json")

	resp, err := env.httpProvider.RoundTrip(req)
	if err != nil {
		return trace.Wrap(err)
	}
	resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden:
		return trace.BadParameter("invalid token")
	case http.StatusInternalServerError:
		return trace.BadParameter("internal server error")
	default:
		return trace.BadParameter("unexpected status code %v", resp.StatusCode)
	}
	return nil
}
