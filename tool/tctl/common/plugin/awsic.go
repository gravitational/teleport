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
	"maps"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	apicommon "github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/lib/utils"
	toolcommon "github.com/gravitational/teleport/tool/common"
)

type awsICArgs struct {
	cmd                  *kingpin.CmdClause
	defaultOwners        []string
	scimToken            string
	scimURL              string
	region               string
	arn                  string
	useSystemCredentials bool
	userOrigin           string
	userLabels           toolcommon.Labels
	groupNameFilters     []string
	accountNameFilters   []string
	accountIDFilters     []string
}

func (a *awsICArgs) validate() error {
	if !a.useSystemCredentials {
		return trace.BadParameter("only AWS Local system credentials are supported")
	}
	return nil
}

// parseAWSICNameFilters validates that all elements of the supplied [names] slice
// are valid regexes or globs and wraps them in [types.AWSICResourceFilter]s for
// inclusion in a [types.PluginAWSICSettings].
//
// We are using a manual validator here rather than the canonical one defined
// in the AWS IC integration itself, because those filter tools are not
// available to OSS builds of tctl.
func parseAWSICNameFilters(names []string) ([]*types.AWSICResourceFilter, error) {
	var filters []*types.AWSICResourceFilter
	for _, n := range names {
		if _, err := utils.CompileExpression(n); err != nil {
			return nil, trace.Wrap(err)
		}
		filters = append(filters, &types.AWSICResourceFilter{
			Include: &types.AWSICResourceFilter_NameRegex{NameRegex: n},
		})
	}
	return filters, nil
}

func (a *awsICArgs) parseGroupFilters() ([]*types.AWSICResourceFilter, error) {
	return parseAWSICNameFilters(a.groupNameFilters)
}

func (a *awsICArgs) parseAccountFilters() ([]*types.AWSICResourceFilter, error) {
	filters, err := parseAWSICNameFilters(a.accountNameFilters)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, id := range a.accountIDFilters {
		filters = append(filters, &types.AWSICResourceFilter{
			Include: &types.AWSICResourceFilter_Id{Id: id},
		})
	}

	return filters, nil
}

func (a *awsICArgs) parseUserFilters() ([]*types.AWSICUserSyncFilter, error) {
	var result []*types.AWSICUserSyncFilter

	labels := make(toolcommon.Labels)
	maps.Copy(labels, a.userLabels)
	if a.userOrigin != "" {
		labels[types.OriginLabel] = a.userOrigin
	}

	if len(labels) > 0 {
		result = append(result, &types.AWSICUserSyncFilter{Labels: labels})
	}

	return result, nil
}

func (p *PluginsCommand) initInstallAWSIC(parent *kingpin.CmdClause) {
	p.install.awsIC.cmd = parent.Command("awsic", "Install an AWS Identity Center integration.")
	cmd := p.install.awsIC.cmd
	cmd.Flag("default-owner", "List of Teleport users that are default owners for the imported access lists. Multiple flags allowed.").Required().StringsVar(&p.install.awsIC.defaultOwners)
	cmd.Flag("url", "AWS Identity Center SCIM provisioning endpoint").Required().StringVar(&p.install.awsIC.scimURL)
	cmd.Flag("token", "AWS Identify Center SCIM provisioning token.").Required().StringVar(&p.install.awsIC.scimToken)
	cmd.Flag("region", "AWS Identity center instance region").Required().StringVar(&p.install.awsIC.region)
	cmd.Flag("arn", "AWS Identify center instance ARN").Required().StringVar(&p.install.awsIC.arn)
	cmd.Flag("use-system-credentials", "Uses system credentials instead of OIDC.").Default("true").BoolVar(&p.install.awsIC.useSystemCredentials)

	cmd.Flag("user-origin", fmt.Sprintf(`Shorthand for "--user-label %s=ORIGIN"`, types.OriginLabel)).
		PlaceHolder("ORIGIN").
		EnumVar(&p.install.awsIC.userOrigin, types.OriginValues...)

	cmd.Flag("user-label", "Add item to user label filter, in the form \"name=value\". If no labels are supplied, all Teleport users will be provisioned to Identity Center").
		SetValue(&p.install.awsIC.userLabels)

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
	if err := awsICArgs.validate(); err != nil {
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
							BaseUrl: awsICArgs.scimURL,
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
						"aws-ic/scim-api-endpoint": awsICArgs.scimURL,
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
