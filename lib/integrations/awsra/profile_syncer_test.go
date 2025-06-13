//go:build go1.24 && enablesynctest

/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package awsra

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rolesanywhere"
	ratypes "github.com/aws/aws-sdk-go-v2/service/rolesanywhere/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/integrations/awsra/createsession"
	"github.com/gravitational/teleport/lib/utils"
)

/*
This file uses the experimental testing/synctest package introduced with Go 1.24:

    https://go.dev/blog/synctest

When editing this file, you should set GOEXPERIMENT=synctest for your editor/LSP
to ensure that the language server doesn't fail to recognize the package.

This file is also protected by a build tag to ensure that `go test` doesn't fail
for users who haven't set the environment variable.
*/

func TestProfileSyncerTestAndSetDefaults(t *testing.T) {
	baseParams := func() *AWSRolesAnywherProfileSyncerParams {
		return &AWSRolesAnywherProfileSyncerParams{
			KeyStoreManager:   keystore.NewSoftwareKeystoreForTests(t),
			Cache:             &mockCache{},
			AppServerUpserter: &mockCache{},
		}
	}

	for _, tt := range []struct {
		name       string
		params     *AWSRolesAnywherProfileSyncerParams
		errCheck   require.ErrorAssertionFunc
		valueCheck func(*testing.T, *AWSRolesAnywherProfileSyncerParams)
	}{
		{
			name:     "default values",
			params:   baseParams(),
			errCheck: require.NoError,
			valueCheck: func(t *testing.T, p *AWSRolesAnywherProfileSyncerParams) {
				require.Equal(t, 5*time.Minute, p.SyncPollInterval)
				require.NotNil(t, p.Logger)
				require.NotNil(t, p.Clock)
				require.NotEmpty(t, p.HostUUID)
			},
		},
		{
			name: "missing key store manager",
			params: func() *AWSRolesAnywherProfileSyncerParams {
				p := baseParams()
				p.KeyStoreManager = nil
				return p
			}(),
			errCheck: require.Error,
		},
		{
			name: "missing cache client",
			params: func() *AWSRolesAnywherProfileSyncerParams {
				p := baseParams()
				p.Cache = nil
				return p
			}(),
			errCheck: require.Error,
		},
		{
			name: "missing AppServerUpserter",
			params: func() *AWSRolesAnywherProfileSyncerParams {
				p := baseParams()
				p.AppServerUpserter = nil
				return p
			}(),
			errCheck: require.Error,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.params.checkAndSetDefaults()
			if tt.errCheck != nil {
				tt.errCheck(t, err)
			}
			if tt.valueCheck != nil {
				tt.valueCheck(t, tt.params)
			}
		})
	}
}

func TestRunAWSRolesAnywherProfileSyncer(t *testing.T) {
	awsRolesAnywhereIntegration := func(t *testing.T, name string, syncEnabled bool) types.Integration {
		t.Helper()

		ig, err := types.NewIntegrationAWSRA(types.Metadata{Name: name}, &types.AWSRAIntegrationSpecV1{
			TrustAnchorARN: "arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/ExampleTrustAnchor",
			ProfileSyncConfig: &types.AWSRolesAnywhereProfileSyncConfig{
				Enabled:                       syncEnabled,
				ProfileARN:                    "arn:aws:rolesanywhere:eu-west-2:123456789012:profile/uuid2",
				ProfileAcceptsRoleSessionName: true,
				RoleARN:                       "arn:aws:iam::123456789012:role/SyncRole",
			},
		})
		require.NoError(t, err)
		return ig
	}

	mockCreateSession := func(ctx context.Context, req createsession.CreateSessionRequest) (*createsession.CreateSessionResponse, error) {
		return &createsession.CreateSessionResponse{
			Version:         1,
			AccessKeyID:     "access-key-id",
			SecretAccessKey: "secret-access-key",
			SessionToken:    "session-token",
			Expiration:      time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339),
		}, nil
	}

	exampleProfile := ratypes.ProfileDetail{
		Name:                  aws.String("ExampleProfile"),
		ProfileArn:            aws.String("arn:aws:rolesanywhere:eu-west-2:123456789012:profile/uuid1"),
		Enabled:               aws.Bool(true),
		AcceptRoleSessionName: aws.Bool(true),
	}

	syncProfile := ratypes.ProfileDetail{
		Name:                  aws.String("SyncProfile"),
		ProfileArn:            aws.String("arn:aws:rolesanywhere:eu-west-2:123456789012:profile/uuid2"),
		Enabled:               aws.Bool(true),
		AcceptRoleSessionName: aws.Bool(true),
	}

	disabledProfile := ratypes.ProfileDetail{
		Name:                  aws.String("SyncProfile"),
		ProfileArn:            aws.String("arn:aws:rolesanywhere:eu-west-2:123456789012:profile/uuid3"),
		Enabled:               aws.Bool(false),
		AcceptRoleSessionName: aws.Bool(true),
	}

	exampleProfileTags := map[string][]ratypes.Tag{
		aws.ToString(exampleProfile.ProfileArn): {
			{Key: aws.String("MyTagKey"), Value: aws.String("my-tag-value")},
		},
	}

	syncEnabled := true
	integrationWithProfileSync := awsRolesAnywhereIntegration(t, "test-integration", syncEnabled)

	syncDisabled := false
	integrationWithoutProfileSync := awsRolesAnywhereIntegration(t, "test-integration-no-profile-sync", syncDisabled)

	baseServerClient := func(t *testing.T) *mockCache {
		t.Helper()
		return &mockCache{
			integrations: []types.Integration{
				integrationWithProfileSync,
				integrationWithoutProfileSync,
			},
			ca: newCertAuthority(t, types.AWSRACA, "cluster-name"),
		}
	}

	baseParams := func(serverClient *mockCache) AWSRolesAnywherProfileSyncerParams {
		return AWSRolesAnywherProfileSyncerParams{
			KeyStoreManager:   keystore.NewSoftwareKeystoreForTests(t),
			Cache:             serverClient,
			AppServerUpserter: serverClient,
			Logger:            utils.NewSlogLoggerForTests(),
			createSession:     mockCreateSession,
		}
	}

	t.Run("sync profile and disabled profiles are skipped", func(t *testing.T) {
		serverClient := baseServerClient(t)

		params := baseParams(serverClient)
		params.rolesAnywhereClient = &mockRolesAnywhereClient{
			profiles: []ratypes.ProfileDetail{
				syncProfile,
				disabledProfile,
			},
			tags: exampleProfileTags,
		}

		synctest.Run(func() {
			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				err := RunAWSRolesAnywherProfileSyncer(ctx, params)
				assert.NoError(t, err)
			}()

			// Wait for the 1st profile sync iteration.
			synctest.Wait()
			cancel()

			require.Len(t, serverClient.appServers, 0)
		})
	})

	t.Run("app server is created", func(t *testing.T) {
		serverClient := baseServerClient(t)

		params := baseParams(serverClient)
		params.rolesAnywhereClient = &mockRolesAnywhereClient{
			profiles: []ratypes.ProfileDetail{
				syncProfile,
				disabledProfile,
				exampleProfile,
			},
			tags: exampleProfileTags,
		}

		synctest.Run(func() {
			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				err := RunAWSRolesAnywherProfileSyncer(ctx, params)
				assert.NoError(t, err)
			}()

			// Wait for the 1st profile sync iteration.
			synctest.Wait()
			cancel()

			require.Len(t, serverClient.appServers, 1)
			appServer := serverClient.appServers[0]
			require.Equal(t, "ExampleProfile-test-integration", appServer.GetName())
			require.Equal(t, "123456789012", appServer.GetApp().GetAWSAccountID())
			require.True(t, appServer.GetApp().GetAWSRolesAnywhereAcceptRoleSessionName())
			require.Equal(t, "arn:aws:rolesanywhere:eu-west-2:123456789012:profile/uuid1", appServer.GetApp().GetAWSRolesAnywhereProfileARN())
			require.Equal(t, map[string]string{
				"aws/MyTagKey":            "my-tag-value",
				"aws_account_id":          "123456789012",
				"teleport.dev/account-id": "123456789012",
				"teleport.dev/aws-roles-anywhere-profile-arn": "arn:aws:rolesanywhere:eu-west-2:123456789012:profile/uuid1",
				"teleport.dev/integration":                    "test-integration",
			}, appServer.GetAllLabels())
		})
	})

	t.Run("app server name is sourced from TeleportApplicationName Profile Tag, if set", func(t *testing.T) {
		serverClient := baseServerClient(t)

		params := baseParams(serverClient)
		tags := map[string][]ratypes.Tag{
			aws.ToString(exampleProfile.ProfileArn): {
				{Key: aws.String("TeleportApplicationName"), Value: aws.String("ProfileCustomName")},
			},
		}
		params.rolesAnywhereClient = &mockRolesAnywhereClient{
			profiles: []ratypes.ProfileDetail{
				exampleProfile,
			},
			tags: tags,
		}

		synctest.Run(func() {
			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				err := RunAWSRolesAnywherProfileSyncer(ctx, params)
				assert.NoError(t, err)
			}()

			// Wait for the 1st profile sync iteration.
			synctest.Wait()
			cancel()

			require.Len(t, serverClient.appServers, 1)
			appServer := serverClient.appServers[0]
			require.Equal(t, "ProfileCustomName", appServer.GetName())
			require.Equal(t, "123456789012", appServer.GetApp().GetAWSAccountID())
			require.True(t, appServer.GetApp().GetAWSRolesAnywhereAcceptRoleSessionName())
			require.Equal(t, "arn:aws:rolesanywhere:eu-west-2:123456789012:profile/uuid1", appServer.GetApp().GetAWSRolesAnywhereProfileARN())
			require.Equal(t, map[string]string{
				"aws/TeleportApplicationName":                 "ProfileCustomName",
				"aws_account_id":                              "123456789012",
				"teleport.dev/account-id":                     "123456789012",
				"teleport.dev/aws-roles-anywhere-profile-arn": "arn:aws:rolesanywhere:eu-west-2:123456789012:profile/uuid1",
				"teleport.dev/integration":                    "test-integration",
			}, appServer.GetAllLabels())
		})
	})
}

type mockRolesAnywhereClient struct {
	profiles []ratypes.ProfileDetail
	tags     map[string][]ratypes.Tag
}

// Lists all profiles in the authenticated account and Amazon Web Services Region.
func (m *mockRolesAnywhereClient) ListProfiles(ctx context.Context, params *rolesanywhere.ListProfilesInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.ListProfilesOutput, error) {
	return &rolesanywhere.ListProfilesOutput{
		Profiles: m.profiles,
	}, nil
}

// Lists the tags attached to the resource.
func (m *mockRolesAnywhereClient) ListTagsForResource(ctx context.Context, params *rolesanywhere.ListTagsForResourceInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.ListTagsForResourceOutput, error) {
	tags := m.tags[aws.ToString(params.ResourceArn)]
	return &rolesanywhere.ListTagsForResourceOutput{
		Tags: tags,
	}, nil
}
