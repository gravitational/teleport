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
	"fmt"
	"strconv"
	"testing"
	"testing/synctest"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/rolesanywhere"
	ratypes "github.com/aws/aws-sdk-go-v2/service/rolesanywhere/types"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	integrationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/integrations/awsra/createsession"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestProfileSyncerTestAndSetDefaults(t *testing.T) {
	cache := &mockCache{}
	keyStoreManager, err := keystore.NewManager(t.Context(), &servicecfg.KeystoreConfig{}, &keystore.Options{
		ClusterName:          &types.ClusterNameV2{Metadata: types.Metadata{Name: "cluster-name"}},
		AuthPreferenceGetter: cache,
	})
	require.NoError(t, err)

	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	baseParams := func() *AWSRolesAnywhereProfileSyncerParams {
		return &AWSRolesAnywhereProfileSyncerParams{
			KeyStoreManager:   keyStoreManager,
			Backend:           backend,
			Cache:             &mockCache{},
			StatusReporter:    &mockCache{},
			AppServerUpserter: &mockCache{},
		}
	}

	for _, tt := range []struct {
		name       string
		params     *AWSRolesAnywhereProfileSyncerParams
		errCheck   require.ErrorAssertionFunc
		valueCheck func(*testing.T, *AWSRolesAnywhereProfileSyncerParams)
	}{
		{
			name:     "default values",
			params:   baseParams(),
			errCheck: require.NoError,
			valueCheck: func(t *testing.T, p *AWSRolesAnywhereProfileSyncerParams) {
				require.Equal(t, 5*time.Minute, p.SyncPollInterval)
				require.NotNil(t, p.Logger)
				require.NotNil(t, p.Clock)
				require.NotEmpty(t, p.HostUUID)
			},
		},
		{
			name: "missing key store manager",
			params: func() *AWSRolesAnywhereProfileSyncerParams {
				p := baseParams()
				p.KeyStoreManager = nil
				return p
			}(),
			errCheck: require.Error,
		},
		{
			name: "missing backend",
			params: func() *AWSRolesAnywhereProfileSyncerParams {
				p := baseParams()
				p.Backend = nil
				return p
			}(),
			errCheck: require.Error,
		},
		{
			name: "missing cache client",
			params: func() *AWSRolesAnywhereProfileSyncerParams {
				p := baseParams()
				p.Cache = nil
				return p
			}(),
			errCheck: require.Error,
		},
		{
			name: "missing AppServerUpserter",
			params: func() *AWSRolesAnywhereProfileSyncerParams {
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
			integrations: map[string]types.Integration{
				integrationWithProfileSync.GetName():    integrationWithProfileSync,
				integrationWithoutProfileSync.GetName(): integrationWithoutProfileSync,
			},
			ca: newCertAuthority(t, types.AWSRACA, "cluster-name"),
		}
	}

	cache := &mockCache{}
	keyStoreManager, err := keystore.NewManager(t.Context(), &servicecfg.KeystoreConfig{}, &keystore.Options{
		ClusterName:          &types.ClusterNameV2{Metadata: types.Metadata{Name: "cluster-name"}},
		AuthPreferenceGetter: cache,
	})
	require.NoError(t, err)

	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	baseParams := func(serverClient *mockCache) AWSRolesAnywhereProfileSyncerParams {
		return AWSRolesAnywhereProfileSyncerParams{
			KeyStoreManager:   keyStoreManager,
			Cache:             serverClient,
			StatusReporter:    serverClient,
			Backend:           backend,
			AppServerUpserter: serverClient,
			Logger:            logtest.NewLogger(),
			createSession:     mockCreateSession,
		}
	}

	t.Run("sync profile and disabled profiles are skipped", func(t *testing.T) {
		serverClient := baseServerClient(t)

		params := baseParams(serverClient)
		params.rolesAnywhereClient = &mockRolesAnywhereClient{
			pages: [][]ratypes.ProfileDetail{{
				syncProfile,
				disabledProfile,
			}},
			tags: exampleProfileTags,
		}

		synctest.Test(t, func(t *testing.T) {
			go func() {
				err := RunAWSRolesAnywhereProfileSyncerWhileLocked(t.Context(), params)
				assert.NoError(t, err)
			}()

			// Wait for the 1st profile sync iteration.
			synctest.Wait()

			require.Empty(t, serverClient.appServers)
		})
	})

	t.Run("app server is created", func(t *testing.T) {
		serverClient := baseServerClient(t)

		params := baseParams(serverClient)
		params.rolesAnywhereClient = &mockRolesAnywhereClient{
			pages: [][]ratypes.ProfileDetail{{
				syncProfile,
				disabledProfile,
				exampleProfile,
			}},
			tags: exampleProfileTags,
		}

		synctest.Test(t, func(t *testing.T) {
			go func() {
				err := RunAWSRolesAnywhereProfileSyncerWhileLocked(t.Context(), params)
				assert.NoError(t, err)
			}()

			// Wait for the 1st profile sync iteration.
			synctest.Wait()

			require.Len(t, serverClient.appServers, 1)
			appServer := serverClient.appServers[0]
			require.Equal(t, "exampleprofile-test-integration", appServer.GetName())
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

			status := serverClient.integrations[integrationWithProfileSync.GetName()].GetStatus()
			require.NotNil(t, status)
			lastSyncSummary := status.AWSRolesAnywhere.LastProfileSync
			require.Equal(t, types.IntegrationAWSRolesAnywhereProfileSyncStatusSuccess, lastSyncSummary.Status)
			require.NotEmpty(t, lastSyncSummary.StartTime)
			require.NotEmpty(t, lastSyncSummary.EndTime)
			require.Equal(t, int32(1), lastSyncSummary.SyncedProfiles)
			require.Empty(t, lastSyncSummary.ErrorMessage)
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
			pages: [][]ratypes.ProfileDetail{{
				exampleProfile,
			}},
			tags: tags,
		}

		synctest.Test(t, func(t *testing.T) {
			go func() {
				err := RunAWSRolesAnywhereProfileSyncerWhileLocked(t.Context(), params)
				assert.NoError(t, err)
			}()

			// Wait for the 1st profile sync iteration.
			synctest.Wait()

			require.Len(t, serverClient.appServers, 1)
			appServer := serverClient.appServers[0]
			require.Equal(t, "profilecustomname-test-integration", appServer.GetName())
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

	t.Run("errors are reported in the integration status", func(t *testing.T) {
		serverClient := baseServerClient(t)

		params := baseParams(serverClient)
		tags := map[string][]ratypes.Tag{
			aws.ToString(exampleProfile.ProfileArn): {
				{Key: aws.String("TeleportApplicationName"), Value: aws.String("___")},
			},
		}
		params.rolesAnywhereClient = &mockRolesAnywhereClient{
			pages: [][]ratypes.ProfileDetail{{
				exampleProfile,
			}},
			tags: tags,
		}

		synctest.Test(t, func(t *testing.T) {
			go func() {
				err := RunAWSRolesAnywhereProfileSyncerWhileLocked(t.Context(), params)
				assert.NoError(t, err)
			}()

			// Wait for the 1st profile sync iteration.
			synctest.Wait()

			status := serverClient.integrations[integrationWithProfileSync.GetName()].GetStatus()
			require.NotNil(t, status)
			lastSyncSummary := status.AWSRolesAnywhere.LastProfileSync
			require.Equal(t, types.IntegrationAWSRolesAnywhereProfileSyncStatusError, lastSyncSummary.Status)
			require.NotEmpty(t, lastSyncSummary.StartTime)
			require.NotEmpty(t, lastSyncSummary.EndTime)
			require.Equal(t, int32(0), lastSyncSummary.SyncedProfiles)
			require.NotEmpty(t, lastSyncSummary.ErrorMessage)
		})
	})

	t.Run("colliding sanitized names report an error", func(t *testing.T) {
		serverClient := baseServerClient(t)

		// prod_ops and prod-ops sanitize to the same name; the second
		// must error rather than overwrite the first.
		profileA := ratypes.ProfileDetail{
			Name:                  aws.String("prod_ops"),
			ProfileArn:            aws.String("arn:aws:rolesanywhere:eu-west-2:123456789012:profile/uuid-a"),
			Enabled:               aws.Bool(true),
			AcceptRoleSessionName: aws.Bool(true),
		}
		profileB := ratypes.ProfileDetail{
			Name:                  aws.String("prod-ops"),
			ProfileArn:            aws.String("arn:aws:rolesanywhere:eu-west-2:123456789012:profile/uuid-b"),
			Enabled:               aws.Bool(true),
			AcceptRoleSessionName: aws.Bool(true),
		}

		params := baseParams(serverClient)
		params.rolesAnywhereClient = &mockRolesAnywhereClient{
			pages: [][]ratypes.ProfileDetail{{profileA, profileB}},
			tags:  map[string][]ratypes.Tag{},
		}

		synctest.Test(t, func(t *testing.T) {
			go func() {
				err := RunAWSRolesAnywhereProfileSyncerWhileLocked(t.Context(), params)
				assert.NoError(t, err)
			}()

			synctest.Wait()

			require.Len(t, serverClient.appServers, 1)
			require.Equal(t, "prod-ops-test-integration", serverClient.appServers[0].GetName())
			require.Equal(t, aws.ToString(profileA.ProfileArn), serverClient.appServers[0].GetApp().GetAWSRolesAnywhereProfileARN())

			status := serverClient.integrations[integrationWithProfileSync.GetName()].GetStatus()
			require.NotNil(t, status)
			lastSyncSummary := status.AWSRolesAnywhere.LastProfileSync
			require.Equal(t, types.IntegrationAWSRolesAnywhereProfileSyncStatusError, lastSyncSummary.Status)
			require.Equal(t, int32(1), lastSyncSummary.SyncedProfiles)
			require.Contains(t, lastSyncSummary.ErrorMessage, "prod-ops-test-integration")
			require.Contains(t, lastSyncSummary.ErrorMessage, "prod_ops")
		})
	})

	t.Run("colliding sanitized names across pages report an error", func(t *testing.T) {
		// Returns the collision across two pages, exercising the
		// cross-page detection path (seenAppNames declared outside
		// the page loop).
		serverClient := baseServerClient(t)

		profileA := ratypes.ProfileDetail{
			Name:                  aws.String("prod_ops"),
			ProfileArn:            aws.String("arn:aws:rolesanywhere:eu-west-2:123456789012:profile/uuid-a"),
			Enabled:               aws.Bool(true),
			AcceptRoleSessionName: aws.Bool(true),
		}
		profileB := ratypes.ProfileDetail{
			Name:                  aws.String("prod-ops"),
			ProfileArn:            aws.String("arn:aws:rolesanywhere:eu-west-2:123456789012:profile/uuid-b"),
			Enabled:               aws.Bool(true),
			AcceptRoleSessionName: aws.Bool(true),
		}

		params := baseParams(serverClient)
		params.rolesAnywhereClient = &mockRolesAnywhereClient{
			pages: [][]ratypes.ProfileDetail{{profileA}, {profileB}},
			tags:  map[string][]ratypes.Tag{},
		}

		synctest.Test(t, func(t *testing.T) {
			go func() {
				err := RunAWSRolesAnywhereProfileSyncerWhileLocked(t.Context(), params)
				assert.NoError(t, err)
			}()

			synctest.Wait()

			require.Len(t, serverClient.appServers, 1)
			require.Equal(t, "prod-ops-test-integration", serverClient.appServers[0].GetName())
			require.Equal(t, aws.ToString(profileA.ProfileArn), serverClient.appServers[0].GetApp().GetAWSRolesAnywhereProfileARN())

			status := serverClient.integrations[integrationWithProfileSync.GetName()].GetStatus()
			require.NotNil(t, status)
			lastSyncSummary := status.AWSRolesAnywhere.LastProfileSync
			require.Equal(t, types.IntegrationAWSRolesAnywhereProfileSyncStatusError, lastSyncSummary.Status)
			require.Equal(t, int32(1), lastSyncSummary.SyncedProfiles)
			require.Contains(t, lastSyncSummary.ErrorMessage, "prod-ops-test-integration")
			require.Contains(t, lastSyncSummary.ErrorMessage, "prod_ops")
		})
	})

	t.Run("app server console URL is partition-specific", func(t *testing.T) {
		for _, tt := range []struct {
			name          string
			integration   string
			appName       string
			syncProfile   string
			appProfile    string
			trustAnchor   string
			roleARN       string
			expectedURI   string
			expectedAppID string
		}{
			{
				name:          "govcloud",
				integration:   "govcloud-integration",
				appName:       "GovProfile",
				syncProfile:   "arn:aws-us-gov:rolesanywhere:us-gov-west-1:123456789012:profile/sync-profile",
				appProfile:    "arn:aws-us-gov:rolesanywhere:us-gov-west-1:123456789012:profile/uuid-gov",
				trustAnchor:   "arn:aws-us-gov:rolesanywhere:us-gov-west-1:123456789012:trust-anchor/ExampleTrustAnchor",
				roleARN:       "arn:aws-us-gov:iam::123456789012:role/SyncRole",
				expectedURI:   constants.AWSUSGovConsoleURL,
				expectedAppID: "govprofile-govcloud-integration",
			},
			{
				name:          "china",
				integration:   "china-integration",
				appName:       "ChinaProfile",
				syncProfile:   "arn:aws-cn:rolesanywhere:cn-north-1:123456789012:profile/sync-profile",
				appProfile:    "arn:aws-cn:rolesanywhere:cn-north-1:123456789012:profile/uuid-cn",
				trustAnchor:   "arn:aws-cn:rolesanywhere:cn-north-1:123456789012:trust-anchor/ExampleTrustAnchor",
				roleARN:       "arn:aws-cn:iam::123456789012:role/SyncRole",
				expectedURI:   constants.AWSCNConsoleURL,
				expectedAppID: "chinaprofile-china-integration",
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				integration, err := types.NewIntegrationAWSRA(types.Metadata{Name: tt.integration}, &types.AWSRAIntegrationSpecV1{
					TrustAnchorARN: tt.trustAnchor,
					ProfileSyncConfig: &types.AWSRolesAnywhereProfileSyncConfig{
						Enabled:                       true,
						ProfileARN:                    tt.syncProfile,
						ProfileAcceptsRoleSessionName: true,
						RoleARN:                       tt.roleARN,
					},
				})
				require.NoError(t, err)

				serverClient := &mockCache{
					integrations: map[string]types.Integration{
						integration.GetName(): integration,
					},
					ca: newCertAuthority(t, types.AWSRACA, "cluster-name"),
				}

				syncProfile := ratypes.ProfileDetail{
					Name:                  aws.String("SyncProfile"),
					ProfileArn:            aws.String(tt.syncProfile),
					Enabled:               aws.Bool(true),
					AcceptRoleSessionName: aws.Bool(true),
				}

				appProfile := ratypes.ProfileDetail{
					Name:                  aws.String(tt.appName),
					ProfileArn:            aws.String(tt.appProfile),
					Enabled:               aws.Bool(true),
					AcceptRoleSessionName: aws.Bool(true),
				}

				params := baseParams(serverClient)
				params.rolesAnywhereClient = &mockRolesAnywhereClient{
					pages: [][]ratypes.ProfileDetail{{
						syncProfile,
						appProfile,
					}},
					tags: map[string][]ratypes.Tag{},
				}

				synctest.Test(t, func(t *testing.T) {
					go func() {
						err := RunAWSRolesAnywhereProfileSyncerWhileLocked(t.Context(), params)
						assert.NoError(t, err)
					}()

					synctest.Wait()

					require.Len(t, serverClient.appServers, 1)
					appServer := serverClient.appServers[0]
					require.Equal(t, tt.expectedAppID, appServer.GetName())
					require.Equal(t, tt.expectedURI, appServer.GetApp().GetURI())
					require.Equal(t, tt.appProfile, appServer.GetApp().GetAWSRolesAnywhereProfileARN())

					status := serverClient.integrations[integration.GetName()].GetStatus()
					require.NotNil(t, status)
					lastSyncSummary := status.AWSRolesAnywhere.LastProfileSync
					require.Equal(t, types.IntegrationAWSRolesAnywhereProfileSyncStatusSuccess, lastSyncSummary.Status)
					require.NotEmpty(t, lastSyncSummary.StartTime)
					require.NotEmpty(t, lastSyncSummary.EndTime)
					require.Equal(t, int32(1), lastSyncSummary.SyncedProfiles)
					require.Empty(t, lastSyncSummary.ErrorMessage)
				})
			})
		}
	})

	t.Run("invalid profile ARN reports error status", func(t *testing.T) {
		serverClient := baseServerClient(t)

		invalidProfile := ratypes.ProfileDetail{
			Name:                  aws.String("InvalidProfile"),
			ProfileArn:            aws.String("not-an-arn"),
			Enabled:               aws.Bool(true),
			AcceptRoleSessionName: aws.Bool(true),
		}

		params := baseParams(serverClient)
		params.rolesAnywhereClient = &mockRolesAnywhereClient{
			pages: [][]ratypes.ProfileDetail{{
				syncProfile,
				invalidProfile,
			}},
			tags: map[string][]ratypes.Tag{},
		}

		synctest.Test(t, func(t *testing.T) {
			go func() {
				err := RunAWSRolesAnywhereProfileSyncerWhileLocked(t.Context(), params)
				assert.NoError(t, err)
			}()

			synctest.Wait()

			require.Empty(t, serverClient.appServers)

			status := serverClient.integrations[integrationWithProfileSync.GetName()].GetStatus()
			require.NotNil(t, status)
			lastSyncSummary := status.AWSRolesAnywhere.LastProfileSync
			require.Equal(t, types.IntegrationAWSRolesAnywhereProfileSyncStatusError, lastSyncSummary.Status)
			require.NotEmpty(t, lastSyncSummary.StartTime)
			require.NotEmpty(t, lastSyncSummary.EndTime)
			require.Equal(t, int32(0), lastSyncSummary.SyncedProfiles)
			require.NotEmpty(t, lastSyncSummary.ErrorMessage)
		})
	})
}

func TestSanitizeProfileName(t *testing.T) {
	// Inputs and wants are raw profile.Name; the integration suffix
	// is appended later by convertProfile.
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "already valid",
			input: "prod-ops",
			want:  "prod-ops",
		},
		{
			name:  "underscore replaced",
			input: "prod_ops",
			want:  "prod-ops",
		},
		{
			name:  "space replaced",
			input: "my profile",
			want:  "my-profile",
		},
		{
			name:  "uppercase lowercased",
			input: "ProdOps",
			want:  "prodops",
		},
		{
			name:  "leading underscore stripped",
			input: "_foo",
			want:  "foo",
		},
		{
			name:  "consecutive invalid chars become consecutive hyphens",
			input: "foo__bar",
			want:  "foo--bar",
		},
		{
			name:  "dotted name preserved",
			input: "env.prod",
			want:  "env.prod",
		},
		{
			name:  "underscore adjacent to dot",
			input: "env_.prod_",
			want:  "env.prod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, sanitizeProfileName(tt.input))
		})
	}
}

func TestConvertProfile(t *testing.T) {
	tests := []struct {
		name        string
		profileName string
		tags        map[string]string
		wantAppName string
	}{
		{
			name:        "valid name unchanged",
			profileName: "ExampleProfile",
			wantAppName: "exampleprofile-test-integration",
		},
		{
			name:        "underscores in profile name replaced with hyphens",
			profileName: "prod_ops",
			wantAppName: "prod-ops-test-integration",
		},
		{
			name:        "spaces in profile name replaced with hyphens",
			profileName: "my profile",
			wantAppName: "my-profile-test-integration",
		},
		{
			name:        "override tag sanitized",
			profileName: "ExampleProfile",
			tags:        map[string]string{types.AWSRolesAnywhereProfileNameOverrideLabel: "custom_override name"},
			// The integration suffix is preserved on override to
			// prevent a single-profile tagger from picking a name
			// that collides with a different integration.
			wantAppName: "custom-override-name-test-integration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := &integrationv1.RolesAnywhereProfile{
				Name:    tt.profileName,
				Arn:     "arn:aws:rolesanywhere:eu-west-2:123456789012:profile/uuid1",
				Enabled: true,
				Tags:    tt.tags,
			}

			appServer, err := convertProfile(AWSRolesAnywhereProfileSyncerParams{
				Clock:    clockwork.NewFakeClock(),
				HostUUID: "test-host-uuid",
			}, profile, "test-integration", "proxy.example.com")
			require.NoError(t, err)
			require.Equal(t, tt.wantAppName, appServer.GetApp().GetName())
			require.Equal(t, tt.wantAppName, appServer.GetName())
			// Round-trip through ValidateApp so a runtime heartbeat
			// rejection surfaces here at sync time.
			proxyGetter := &mockProxyGetter{addrs: []string{"proxy.example.com:443"}}
			require.NoError(t, services.ValidateApp(appServer.GetApp(), proxyGetter))
		})
	}
}

// mockProxyGetter is a test implementation of services.ProxyGetter.
type mockProxyGetter struct {
	addrs []string
}

func (m *mockProxyGetter) GetProxies() ([]types.Server, error) {
	servers := make([]types.Server, 0, len(m.addrs))
	for _, addr := range m.addrs {
		servers = append(servers, &types.ServerV2{
			Spec: types.ServerSpecV2{PublicAddrs: []string{addr}},
		})
	}
	return servers, nil
}

func (m *mockProxyGetter) ListProxyServers(_ context.Context, _ int, _ string) ([]types.Server, string, error) {
	servers, _ := m.GetProxies()
	return servers, "", nil
}

func TestAWSConsoleURLForARN(t *testing.T) {
	tests := []struct {
		name        string
		inputARN    string
		expectedURL string
	}{
		{
			name:        "GovCloud us-gov-west-1",
			inputARN:    "arn:aws-us-gov:rolesanywhere:us-gov-west-1:123456789012:profile/uuid1",
			expectedURL: constants.AWSUSGovConsoleURL,
		},
		{
			name:        "GovCloud us-gov-east-1",
			inputARN:    "arn:aws-us-gov:rolesanywhere:us-gov-east-1:123456789012:profile/uuid1",
			expectedURL: constants.AWSUSGovConsoleURL,
		},
		{
			name:        "AWS China",
			inputARN:    "arn:aws-cn:rolesanywhere:cn-north-1:123456789012:profile/uuid1",
			expectedURL: "https://console.amazonaws.cn",
		},
		{
			name:        "AWS Standard",
			inputARN:    "arn:aws:rolesanywhere:eu-west-2:123456789012:profile/uuid1",
			expectedURL: "https://console.aws.amazon.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := arn.Parse(tt.inputARN)
			require.NoError(t, err)
			require.Equal(t, tt.expectedURL, awsConsoleURLForARN(parsed))
		})
	}
}

type mockRolesAnywhereClient struct {
	// pages returns one element per ListProfiles call with a NextToken
	// pointing at the following index. Single-page tests set one entry.
	pages [][]ratypes.ProfileDetail
	tags  map[string][]ratypes.Tag
}

// Lists all profiles in the authenticated account and Amazon Web Services Region.
func (m *mockRolesAnywhereClient) ListProfiles(ctx context.Context, params *rolesanywhere.ListProfilesInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.ListProfilesOutput, error) {
	idx := 0
	if params.NextToken != nil {
		n, err := strconv.Atoi(aws.ToString(params.NextToken))
		if err != nil {
			return nil, trace.Wrap(err, "malformed page token %q", aws.ToString(params.NextToken))
		}
		idx = n
	}
	if idx >= len(m.pages) {
		return &rolesanywhere.ListProfilesOutput{}, nil
	}
	var nextToken *string
	if idx+1 < len(m.pages) {
		nextToken = aws.String(fmt.Sprintf("%d", idx+1))
	}
	return &rolesanywhere.ListProfilesOutput{
		Profiles:  m.pages[idx],
		NextToken: nextToken,
	}, nil
}

// Lists the tags attached to the resource.
func (m *mockRolesAnywhereClient) ListTagsForResource(ctx context.Context, params *rolesanywhere.ListTagsForResourceInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.ListTagsForResourceOutput, error) {
	tags := m.tags[aws.ToString(params.ResourceArn)]
	return &rolesanywhere.ListTagsForResourceOutput{
		Tags: tags,
	}, nil
}
