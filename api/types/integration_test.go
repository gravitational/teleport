/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
)

func TestIntegrationJSONMarshalCycle(t *testing.T) {
	aws, err := NewIntegrationAWSOIDC(
		Metadata{Name: "some-integration"},
		&AWSOIDCIntegrationSpecV1{
			RoleARN:     "arn:aws:iam::123456789012:role/DevTeams",
			IssuerS3URI: "s3://my-bucket/my-prefix",
		},
	)
	require.NoError(t, err)

	azure, err := NewIntegrationAzureOIDC(
		Metadata{Name: "some-integration"},
		&AzureOIDCIntegrationSpecV1{
			TenantID: "foo-bar",
			ClientID: "baz-quux",
		},
	)
	require.NoError(t, err)

	awsra, err := NewIntegrationAWSRA(
		Metadata{Name: "some-integration"},
		&AWSRAIntegrationSpecV1{
			TrustAnchorARN: "arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/12345678-1234-1234-1234-123456789012",
		},
	)
	require.NoError(t, err)

	allIntegrations := []*IntegrationV1{aws, azure, awsra}

	for _, ig := range allIntegrations {
		t.Run(ig.SubKind, func(t *testing.T) {
			bs, err := json.Marshal(ig)
			require.NoError(t, err)

			var ig2 IntegrationV1
			err = json.Unmarshal(bs, &ig2)
			require.NoError(t, err)

			require.Equal(t, &ig2, ig)
		})
	}
}

func TestIntegrationCheckAndSetDefaults(t *testing.T) {
	noErrorFunc := func(err error) bool {
		return err == nil
	}

	for _, tt := range []struct {
		name                string
		integration         func(string) (*IntegrationV1, error)
		expectedIntegration func(string) *IntegrationV1
		expectedErrorIs     func(error) bool
	}{
		{
			name: "aws-oidc: valid",
			integration: func(name string) (*IntegrationV1, error) {
				return NewIntegrationAWSOIDC(
					Metadata{
						Name: name,
					},
					&AWSOIDCIntegrationSpecV1{
						RoleARN:     "some arn role",
						IssuerS3URI: "s3://my-issuer/my-prefix",
					},
				)
			},
			expectedIntegration: func(name string) *IntegrationV1 {
				return &IntegrationV1{
					ResourceHeader: ResourceHeader{
						Kind:    KindIntegration,
						SubKind: IntegrationSubKindAWSOIDC,
						Version: V1,
						Metadata: Metadata{
							Name:      name,
							Namespace: defaults.Namespace,
						},
					},
					Spec: IntegrationSpecV1{
						SubKindSpec: &IntegrationSpecV1_AWSOIDC{
							AWSOIDC: &AWSOIDCIntegrationSpecV1{
								RoleARN:     "some arn role",
								IssuerS3URI: "s3://my-issuer/my-prefix",
								Audience:    "",
							},
						},
					},
				}
			},
			expectedErrorIs: noErrorFunc,
		},
		{
			name: "aws-oidc: valid with supported audience value",
			integration: func(name string) (*IntegrationV1, error) {
				return NewIntegrationAWSOIDC(
					Metadata{
						Name: name,
					},
					&AWSOIDCIntegrationSpecV1{
						RoleARN:     "some arn role",
						IssuerS3URI: "s3://my-issuer/my-prefix",
						Audience:    IntegrationAWSOIDCAudienceAWSIdentityCenter,
					},
				)
			},
			expectedIntegration: func(name string) *IntegrationV1 {
				return &IntegrationV1{
					ResourceHeader: ResourceHeader{
						Kind:    KindIntegration,
						SubKind: IntegrationSubKindAWSOIDC,
						Version: V1,
						Metadata: Metadata{
							Name:      name,
							Namespace: defaults.Namespace,
						},
					},
					Spec: IntegrationSpecV1{
						SubKindSpec: &IntegrationSpecV1_AWSOIDC{
							AWSOIDC: &AWSOIDCIntegrationSpecV1{
								RoleARN:     "some arn role",
								IssuerS3URI: "s3://my-issuer/my-prefix",
								Audience:    IntegrationAWSOIDCAudienceAWSIdentityCenter,
							},
						},
					},
				}
			},
			expectedErrorIs: noErrorFunc,
		},
		{
			name: "aws-oidc: error when subkind spec is not provided",
			integration: func(name string) (*IntegrationV1, error) {
				return NewIntegrationAWSOIDC(
					Metadata{
						Name: name,
					},
					nil,
				)
			},
			expectedErrorIs: trace.IsBadParameter,
		},
		{
			name: "aws-oidc: error when issuer is not a valid url",
			integration: func(name string) (*IntegrationV1, error) {
				return NewIntegrationAWSOIDC(
					Metadata{
						Name: name,
					},
					&AWSOIDCIntegrationSpecV1{
						RoleARN:     "some-role",
						IssuerS3URI: "not-a-url",
					},
				)
			},
			expectedErrorIs: trace.IsBadParameter,
		},
		{
			name: "aws-oidc: issuer is not an s3 url",
			integration: func(name string) (*IntegrationV1, error) {
				return NewIntegrationAWSOIDC(
					Metadata{
						Name: name,
					},
					&AWSOIDCIntegrationSpecV1{
						RoleARN:     "some-role",
						IssuerS3URI: "http://localhost:8080",
					},
				)
			},
			expectedErrorIs: trace.IsBadParameter,
		},
		{
			name: "aws-oidc: error when no role is provided",
			integration: func(name string) (*IntegrationV1, error) {
				return NewIntegrationAWSOIDC(
					Metadata{
						Name: name,
					},
					&AWSOIDCIntegrationSpecV1{},
				)
			},
			expectedErrorIs: trace.IsBadParameter,
		},
		{
			name: "aws-oidc: error when invalid audience value is provided",
			integration: func(name string) (*IntegrationV1, error) {
				return NewIntegrationAWSOIDC(
					Metadata{
						Name: name,
					},
					&AWSOIDCIntegrationSpecV1{
						RoleARN:  "some-role",
						Audience: "testvalue",
					},
				)
			},
			expectedErrorIs: trace.IsBadParameter,
		},
		{
			name: "azure-oidc: valid",
			integration: func(name string) (*IntegrationV1, error) {
				return NewIntegrationAzureOIDC(
					Metadata{
						Name: name,
					},
					&AzureOIDCIntegrationSpecV1{
						ClientID: "baz-quux",
						TenantID: "foo-bar",
					},
				)
			},
			expectedIntegration: func(name string) *IntegrationV1 {
				return &IntegrationV1{
					ResourceHeader: ResourceHeader{
						Kind:    KindIntegration,
						SubKind: IntegrationSubKindAzureOIDC,
						Version: V1,
						Metadata: Metadata{
							Name:      name,
							Namespace: defaults.Namespace,
						},
					},
					Spec: IntegrationSpecV1{
						SubKindSpec: &IntegrationSpecV1_AzureOIDC{
							AzureOIDC: &AzureOIDCIntegrationSpecV1{
								ClientID: "baz-quux",
								TenantID: "foo-bar",
							},
						},
					},
				}
			},
			expectedErrorIs: noErrorFunc,
		},
		{
			name: "azure-oidc: error when no tenant id is provided",
			integration: func(name string) (*IntegrationV1, error) {
				return NewIntegrationAzureOIDC(
					Metadata{
						Name: name,
					},
					&AzureOIDCIntegrationSpecV1{
						ClientID: "baz-quux",
					},
				)
			},
			expectedErrorIs: trace.IsBadParameter,
		},
		{
			name: "azure-oidc: error when no client id is provided",
			integration: func(name string) (*IntegrationV1, error) {
				return NewIntegrationAzureOIDC(
					Metadata{
						Name: name,
					},
					&AzureOIDCIntegrationSpecV1{
						TenantID: "foo-bar",
					},
				)
			},
			expectedErrorIs: trace.IsBadParameter,
		},
		{
			name: "github: valid",
			integration: func(name string) (*IntegrationV1, error) {
				return NewIntegrationGitHub(
					Metadata{
						Name: name,
					},
					&GitHubIntegrationSpecV1{
						Organization: "my-org",
					},
				)
			},
			expectedIntegration: func(name string) *IntegrationV1 {
				return &IntegrationV1{
					ResourceHeader: ResourceHeader{
						Kind:    KindIntegration,
						SubKind: IntegrationSubKindGitHub,
						Version: V1,
						Metadata: Metadata{
							Name:      name,
							Namespace: defaults.Namespace,
						},
					},
					Spec: IntegrationSpecV1{
						SubKindSpec: &IntegrationSpecV1_GitHub{
							GitHub: &GitHubIntegrationSpecV1{
								Organization: "my-org",
							},
						},
					},
				}
			},
			expectedErrorIs: noErrorFunc,
		},
		{
			name: "github: error when invalid org is provided",
			integration: func(name string) (*IntegrationV1, error) {
				return NewIntegrationGitHub(
					Metadata{
						Name: name,
					},
					&GitHubIntegrationSpecV1{},
				)
			},
			expectedErrorIs: trace.IsBadParameter,
		},
		{
			name: "aws ra: valid",
			integration: func(name string) (*IntegrationV1, error) {
				return NewIntegrationAWSRA(
					Metadata{
						Name: name,
					},
					&AWSRAIntegrationSpecV1{
						TrustAnchorARN: "arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/12345678-1234-1234-1234-123456789012",
					},
				)
			},
			expectedIntegration: func(name string) *IntegrationV1 {
				return &IntegrationV1{
					ResourceHeader: ResourceHeader{
						Kind:    KindIntegration,
						SubKind: IntegrationSubKindAWSRolesAnywhere,
						Version: V1,
						Metadata: Metadata{
							Name:      name,
							Namespace: defaults.Namespace,
						},
					},
					Spec: IntegrationSpecV1{
						SubKindSpec: &IntegrationSpecV1_AWSRA{
							AWSRA: &AWSRAIntegrationSpecV1{
								TrustAnchorARN:    "arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/12345678-1234-1234-1234-123456789012",
								ProfileSyncConfig: &AWSRolesAnywhereProfileSyncConfig{},
							},
						},
					},
				}
			},
			expectedErrorIs: noErrorFunc,
		},
		{
			name: "aws ra: error when missing trust anchor arn",
			integration: func(name string) (*IntegrationV1, error) {
				return NewIntegrationAWSRA(
					Metadata{
						Name: name,
					},
					&AWSRAIntegrationSpecV1{},
				)
			},
			expectedErrorIs: trace.IsBadParameter,
		},
		{
			name: "aws ra: error when sync is enabled but sync profile is missing",
			integration: func(name string) (*IntegrationV1, error) {
				return NewIntegrationAWSRA(
					Metadata{
						Name: name,
					},
					&AWSRAIntegrationSpecV1{
						TrustAnchorARN: "arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/12345678-1234-1234-1234-123456789012",
						ProfileSyncConfig: &AWSRolesAnywhereProfileSyncConfig{
							Enabled: true,
							RoleARN: "arn:aws:iam::123456789012:role/DevTeams",
						},
					},
				)
			},
			expectedErrorIs: trace.IsBadParameter,
		},
		{
			name: "aws ra: error when sync is enabled but sync role is missing",
			integration: func(name string) (*IntegrationV1, error) {
				return NewIntegrationAWSRA(
					Metadata{
						Name: name,
					},
					&AWSRAIntegrationSpecV1{
						TrustAnchorARN: "arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/12345678-1234-1234-1234-123456789012",
						ProfileSyncConfig: &AWSRolesAnywhereProfileSyncConfig{
							Enabled:    true,
							ProfileARN: "arn:aws:rolesanywhere:eu-west-2:123456789012:profile/12345678-1234-1234-1234-123456789012",
						},
					},
				)
			},
			expectedErrorIs: trace.IsBadParameter,
		},
		{
			name: "aws ra: valid sync configuration",
			integration: func(name string) (*IntegrationV1, error) {
				return NewIntegrationAWSRA(
					Metadata{
						Name: name,
					},
					&AWSRAIntegrationSpecV1{
						TrustAnchorARN: "arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/12345678-1234-1234-1234-123456789012",
						ProfileSyncConfig: &AWSRolesAnywhereProfileSyncConfig{
							Enabled:                       true,
							ProfileARN:                    "arn:aws:rolesanywhere:eu-west-2:123456789012:profile/12345678-1234-1234-1234-123456789012",
							RoleARN:                       "arn:aws:iam::123456789012:role/DevTeams",
							ProfileAcceptsRoleSessionName: true,
						},
					},
				)
			},
			expectedErrorIs: noErrorFunc,
			expectedIntegration: func(name string) *IntegrationV1 {
				return &IntegrationV1{
					ResourceHeader: ResourceHeader{
						Kind:    KindIntegration,
						SubKind: IntegrationSubKindAWSRolesAnywhere,
						Version: V1,
						Metadata: Metadata{
							Name:      name,
							Namespace: defaults.Namespace,
						},
					},
					Spec: IntegrationSpecV1{
						SubKindSpec: &IntegrationSpecV1_AWSRA{
							AWSRA: &AWSRAIntegrationSpecV1{
								TrustAnchorARN: "arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/12345678-1234-1234-1234-123456789012",
								ProfileSyncConfig: &AWSRolesAnywhereProfileSyncConfig{
									Enabled:                       true,
									ProfileARN:                    "arn:aws:rolesanywhere:eu-west-2:123456789012:profile/12345678-1234-1234-1234-123456789012",
									RoleARN:                       "arn:aws:iam::123456789012:role/DevTeams",
									ProfileAcceptsRoleSessionName: true,
								},
							},
						},
					},
				}
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			name := uuid.NewString()
			ig, err := tt.integration(name)
			require.True(t, tt.expectedErrorIs(err), "expected another error", err)
			if err != nil {
				return
			}

			require.Equal(t, tt.expectedIntegration(name), ig)
			require.Contains(t, ig.String(), name)
		})
	}
}
