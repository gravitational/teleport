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

package fixtures

import (
	"testing"

	"github.com/gravitational/teleport/api/types"
)

// NewIdentityCenterPlugin returns a new types.PluginV1 with PluginSpecV1_AwsIc settings.
func NewIdentityCenterPlugin(t *testing.T, serviceProviderName, integrationName string) *types.PluginV1 {
	t.Helper()
	return &types.PluginV1{
		Metadata: types.Metadata{
			Name: types.PluginTypeAWSIdentityCenter,
			Labels: map[string]string{
				types.HostedPluginLabel: "true",
			},
		},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_AwsIc{
				AwsIc: &types.PluginAWSICSettings{
					IntegrationName:         integrationName,
					Region:                  "test-region",
					Arn:                     "test-arn",
					AccessListDefaultOwners: []string{"user1", "user2"},
					ProvisioningSpec: &types.AWSICProvisioningSpec{
						BaseUrl: "https://example.com",
					},
					SamlIdpServiceProviderName: serviceProviderName,
				},
			},
		},
	}
}

// NewIdentityCenterPlugin returns a new types.PluginV1 with PluginSpecV1_Mattermost settings.
func NewMattermostPlugin(t *testing.T) *types.PluginV1 {
	t.Helper()
	return &types.PluginV1{
		SubKind: types.PluginSubkindAccess,
		Metadata: types.Metadata{
			Labels: map[string]string{
				"teleport.dev/hosted-plugin": "true",
			},
			Name: types.PluginTypeMattermost,
		},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Mattermost{
				Mattermost: &types.PluginMattermostSettings{
					ServerUrl:     "https://example.com",
					Channel:       "test_channel",
					Team:          "test_team",
					ReportToEmail: "test@example.com",
				},
			},
		},
		Credentials: &types.PluginCredentialsV1{
			Credentials: &types.PluginCredentialsV1_StaticCredentialsRef{
				StaticCredentialsRef: &types.PluginStaticCredentialsRef{
					Labels: map[string]string{
						"plugin": "mattermost",
					},
				},
			},
		},
	}
}
