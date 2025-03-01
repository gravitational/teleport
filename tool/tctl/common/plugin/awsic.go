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
)

type awsICArgs struct {
	cmd                  *kingpin.CmdClause
	defaultOwners        []string
	scimToken            string
	scimURL              *url.URL
	forceSCIMURL         bool
	region               string
	arn                  string
	useSystemCredentials bool
	userOrigins          []string
	userLabels           []string
	groupNameFilters     []string
	accountNameFilters   []string
	accountIDFilters     []string
}

func (a *awsICArgs) validate(ctx context.Context, log *slog.Logger) error {
	if !awsutils.IsKnownRegion(a.region) {
		return trace.BadParameter("unknown AWS region: %s", a.region)
	}

	if a.scimToken == "" {
		return trace.BadParameter("SCIM token must not be empty")
	}

	if !a.useSystemCredentials {
		return trace.BadParameter("only AWS Local system credentials are supported")
	}

	if err := a.validateSCIMBaseURL(ctx, log); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (a *awsICArgs) validateSCIMBaseURL(ctx context.Context, log *slog.Logger) error {
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

func (a *awsICArgs) parseGroupFilters() (icfilters.Filters, error) {
	var filters []*types.AWSICResourceFilter
	for _, n := range a.groupNameFilters {
		filters = append(filters, &types.AWSICResourceFilter{
			Include: &types.AWSICResourceFilter_NameRegex{NameRegex: n},
		})
	}
	return icfilters.New(filters)
}

func (a *awsICArgs) parseAccountFilters() (icfilters.Filters, error) {
	var filters []*types.AWSICResourceFilter
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

	return icfilters.New(filters)
}

func (a *awsICArgs) parseUserFilters() ([]*types.AWSICUserSyncFilter, error) {
	var result []*types.AWSICUserSyncFilter

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
}

// InstallAWSIC installs AWS Identity Center plugin.
func (p *PluginsCommand) InstallAWSIC(ctx context.Context, args installPluginArgs) error {
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

	accountFilters, err := awsICArgs.parseAccountFilters()
	if err != nil {
		return trace.Wrap(err)
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
					AwsIc: &types.PluginAWSICSettings{
						IntegrationName: apicommon.OriginAWSIdentityCenter,
						Region:          awsICArgs.region,
						Arn:             awsICArgs.arn,
						ProvisioningSpec: &types.AWSICProvisioningSpec{
							BaseUrl: awsICArgs.scimURL.String(),
						},
						AccessListDefaultOwners: awsICArgs.defaultOwners,
						CredentialsSource:       types.AWSICCredentialsSource_AWSIC_CREDENTIALS_SOURCE_SYSTEM,
						UserSyncFilters:         userFilters,
						GroupSyncFilters:        groupFilters,
						AwsAccountsFilters:      accountFilters,
					},
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
