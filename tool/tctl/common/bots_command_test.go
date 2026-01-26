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

package common

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestUpdateBotLogins(t *testing.T) {
	tests := []struct {
		desc          string
		add           string
		set           string
		initialLogins []string
		assert        func(t *testing.T, bot *machineidv1pb.Bot, mask *fieldmaskpb.FieldMask, err error)
	}{
		{
			desc:          "should add and set with existing logins",
			set:           "a,b,c",
			add:           "d,e,e,e,e",
			initialLogins: []string{"a"},
			assert: func(t *testing.T, bot *machineidv1pb.Bot, mask *fieldmaskpb.FieldMask, err error) {
				require.NoError(t, err)
				require.ElementsMatch(t, mask.Paths, []string{"spec.traits"})
				require.ElementsMatch(t, bot.Spec.Traits[0].Values, splitEntries("a,b,c,d,e"))
			},
		},
		{
			desc:          "should not update with no changes",
			set:           "a,b,c",
			add:           "d,e,e,e,e",
			initialLogins: splitEntries("a,b,c,d,e"),
			assert: func(t *testing.T, bot *machineidv1pb.Bot, mask *fieldmaskpb.FieldMask, err error) {
				require.NoError(t, err)
				require.Empty(t, mask.Paths)
				require.ElementsMatch(t, bot.Spec.Traits[0].Values, splitEntries("a,b,c,d,e"))
			},
		},
		{
			desc: "should add with empty initial logins trait",
			set:  "a,b,c",
			assert: func(t *testing.T, bot *machineidv1pb.Bot, mask *fieldmaskpb.FieldMask, err error) {
				require.NoError(t, err)
				require.ElementsMatch(t, mask.Paths, []string{"spec.traits"})
				require.ElementsMatch(t, bot.Spec.Traits[0].Values, splitEntries("a,b,c"))
			},
		},
		{
			desc:          "should remove on set if necessary",
			set:           "a,b,c",
			initialLogins: splitEntries("a,b,c,d,e"),
			assert: func(t *testing.T, bot *machineidv1pb.Bot, mask *fieldmaskpb.FieldMask, err error) {
				require.NoError(t, err)
				require.ElementsMatch(t, mask.Paths, []string{"spec.traits"})
				require.ElementsMatch(t, bot.Spec.Traits[0].Values, splitEntries("a,b,c"))
			},
		},
	}

	for _, tt := range tests {

		const botName = "test"

		t.Run(tt.desc, func(t *testing.T) {
			traits := []*machineidv1pb.Trait{}
			if len(tt.initialLogins) > 0 {
				traits = append(traits, &machineidv1pb.Trait{
					Name:   constants.TraitLogins,
					Values: tt.initialLogins,
				})
			}

			bot := &machineidv1pb.Bot{
				Kind:    types.KindBot,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: botName,
				},
				Spec: &machineidv1pb.BotSpec{
					Roles:  []string{},
					Traits: traits,
				},
			}

			fieldMask, err := fieldmaskpb.New(&machineidv1pb.Bot{})
			require.NoError(t, err)

			cmd := BotsCommand{
				botName:   botName,
				addLogins: tt.add,
				setLogins: tt.set,
			}

			err = cmd.updateBotLogins(context.Background(), bot, fieldMask)
			tt.assert(t, bot, fieldMask, err)
		})
	}
}

// mockAPIClient is a minimal API client used for testing
type mockRoleGetterClient struct {
	*authclient.Client
	roles []string
}

func (m *mockRoleGetterClient) GetRole(ctx context.Context, name string) (types.Role, error) {
	if !slices.Contains(m.roles, name) {
		return nil, trace.NotFound("invalid role %s", name)
	}

	return types.NewRole(name, types.RoleSpecV6{})
}

func TestUpdateBotRoles(t *testing.T) {
	tests := []struct {
		desc         string
		add          string
		set          string
		initialRoles []string
		knownRoles   []string
		assert       func(t *testing.T, bot *machineidv1pb.Bot, mask *fieldmaskpb.FieldMask, err error)
	}{
		{
			desc:         "should add and set without duplicating roles",
			set:          "a,b,c",
			add:          "d,e,e,e,e",
			knownRoles:   splitEntries("a,b,c,d,e"),
			initialRoles: []string{"a"},
			assert: func(t *testing.T, bot *machineidv1pb.Bot, mask *fieldmaskpb.FieldMask, err error) {
				require.NoError(t, err)
				require.ElementsMatch(t, mask.Paths, []string{"spec.roles"})
				require.ElementsMatch(t, bot.Spec.Roles, splitEntries("a,b,c,d,e"))
			},
		},
		{
			desc:         "should not update with no changes",
			set:          "a,b,c",
			add:          "d,e,e,e,e",
			knownRoles:   splitEntries("a,b,c,d,e"),
			initialRoles: splitEntries("a,b,c,d,e"),
			assert: func(t *testing.T, bot *machineidv1pb.Bot, mask *fieldmaskpb.FieldMask, err error) {
				require.NoError(t, err)
				require.Empty(t, mask.Paths)
				require.ElementsMatch(t, bot.Spec.Roles, splitEntries("a,b,c,d,e"))
			},
		},
		{
			desc:         "should remove on set if necessary",
			set:          "a,b,c",
			knownRoles:   splitEntries("a,b,c,d"),
			initialRoles: splitEntries("a,b,c,d"),
			assert: func(t *testing.T, bot *machineidv1pb.Bot, mask *fieldmaskpb.FieldMask, err error) {
				require.NoError(t, err)
				require.ElementsMatch(t, mask.Paths, []string{"spec.roles"})
				require.ElementsMatch(t, bot.Spec.Roles, splitEntries("a,b,c"))
			},
		},
		{
			desc:         "should fail if an unknown role is specified and leave bot unmodified",
			add:          "d",
			knownRoles:   splitEntries("a,b,c"),
			initialRoles: splitEntries("a,b,c"),
			assert: func(t *testing.T, bot *machineidv1pb.Bot, mask *fieldmaskpb.FieldMask, err error) {
				require.True(t, trace.IsNotFound(err))
				require.Empty(t, mask.Paths)
				require.ElementsMatch(t, bot.Spec.Roles, splitEntries("a,b,c"))
			},
		},
	}

	for _, tt := range tests {

		const botName = "test"

		t.Run(tt.desc, func(t *testing.T) {
			mockClient := mockRoleGetterClient{
				roles: tt.knownRoles,
			}

			bot := &machineidv1pb.Bot{
				Kind:    types.KindBot,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: botName,
				},
				Spec: &machineidv1pb.BotSpec{
					Roles: tt.initialRoles,
				},
			}

			fieldMask, err := fieldmaskpb.New(&machineidv1pb.Bot{})
			require.NoError(t, err)

			cmd := BotsCommand{
				botName:  botName,
				addRoles: tt.add,
				botRoles: tt.set,
			}

			err = cmd.updateBotRoles(t.Context(), &mockClient, bot, fieldMask)
			tt.assert(t, bot, fieldMask, err)
		})
	}
}

func TestAddAndListBotInstancesJSON(t *testing.T) {
	dynAddr := helpers.NewDynamicServiceAddr(t)
	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.AuthAddr,
			},
		},
	}
	process := makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.Descriptors))
	ctx := context.Background()
	client, err := testenv.NewDefaultAuthClient(process)
	require.NoError(t, err)

	t.Cleanup(func() { _ = client.Close() })

	tokens, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, pageSize int, pageKey string) ([]types.ProvisionToken, string, error) {
		return client.ListProvisionTokens(ctx, pageSize, pageKey, nil, "")
	}))
	require.NoError(t, err)
	require.Empty(t, tokens)

	// Create an initial bot
	bot, err := client.BotServiceClient().CreateBot(ctx, &machineidv1pb.CreateBotRequest{
		Bot: &machineidv1pb.Bot{
			Kind:    types.KindBot,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "test",
			},
			Spec: &machineidv1pb.BotSpec{},
		},
	})
	require.NoError(t, err)

	// Attempt to add a new instance and ensure a new token was created.
	buf := strings.Builder{}
	cmd := BotsCommand{
		stdout:  &buf,
		format:  teleport.JSON,
		botName: bot.Metadata.Name,
	}
	require.NoError(t, cmd.AddBotInstance(ctx, client))

	response := botJSONResponse{}
	require.NoError(t, json.Unmarshal([]byte(buf.String()), &response))

	_, err = client.GetToken(ctx, response.TokenID)
	require.NoError(t, err)

	// Run the command again to ensure multiple distinct tokens can be created.
	buf.Reset()
	require.NoError(t, cmd.AddBotInstance(ctx, client))

	response2 := botJSONResponse{}
	require.NoError(t, json.Unmarshal([]byte(buf.String()), &response2))

	require.NotEqual(t, response.TokenID, response2.TokenID)

	_, err = client.GetToken(ctx, response2.TokenID)
	require.NoError(t, err)

	buf.Reset()
}

func TestAggregateServiceHealth(t *testing.T) {
	t.Parallel()

	healthy := machineidv1pb.BotInstanceServiceHealth{
		Status: machineidv1pb.BotInstanceHealthStatus_BOT_INSTANCE_HEALTH_STATUS_HEALTHY,
	}
	unhealthy := machineidv1pb.BotInstanceServiceHealth{
		Status: machineidv1pb.BotInstanceHealthStatus_BOT_INSTANCE_HEALTH_STATUS_UNHEALTHY,
	}
	initializing := machineidv1pb.BotInstanceServiceHealth{
		Status: machineidv1pb.BotInstanceHealthStatus_BOT_INSTANCE_HEALTH_STATUS_INITIALIZING,
	}
	unknown := machineidv1pb.BotInstanceServiceHealth{
		Status: machineidv1pb.BotInstanceHealthStatus_BOT_INSTANCE_HEALTH_STATUS_UNSPECIFIED,
	}

	tcs := []struct {
		name      string
		services  []*machineidv1pb.BotInstanceServiceHealth
		hasStatus bool
		status    machineidv1pb.BotInstanceHealthStatus
	}{
		{
			name:      "nil",
			services:  nil,
			hasStatus: false,
			status:    0,
		},
		{
			name:      "empty",
			services:  []*machineidv1pb.BotInstanceServiceHealth{},
			hasStatus: false,
			status:    0,
		},
		{
			name: "one item - healthy",
			services: []*machineidv1pb.BotInstanceServiceHealth{
				&healthy,
			},
			hasStatus: true,
			status:    machineidv1pb.BotInstanceHealthStatus_BOT_INSTANCE_HEALTH_STATUS_HEALTHY,
		},
		{
			name: "one item - unhealthy",
			services: []*machineidv1pb.BotInstanceServiceHealth{
				&unhealthy,
			},
			hasStatus: true,
			status:    machineidv1pb.BotInstanceHealthStatus_BOT_INSTANCE_HEALTH_STATUS_UNHEALTHY,
		},
		{
			name: "one item - initializing",
			services: []*machineidv1pb.BotInstanceServiceHealth{
				&initializing,
			},
			hasStatus: true,
			status:    machineidv1pb.BotInstanceHealthStatus_BOT_INSTANCE_HEALTH_STATUS_INITIALIZING,
		},
		{
			name: "one item - unknown",
			services: []*machineidv1pb.BotInstanceServiceHealth{
				&unknown,
			},
			hasStatus: true,
			status:    machineidv1pb.BotInstanceHealthStatus_BOT_INSTANCE_HEALTH_STATUS_UNSPECIFIED,
		},
		{
			name: "multiple items - healthy",
			services: []*machineidv1pb.BotInstanceServiceHealth{
				&healthy,
				&healthy,
			},
			hasStatus: true,
			status:    machineidv1pb.BotInstanceHealthStatus_BOT_INSTANCE_HEALTH_STATUS_HEALTHY,
		},
		{
			name: "multiple items - unhealthy",
			services: []*machineidv1pb.BotInstanceServiceHealth{
				&unhealthy,
				&healthy,
				&initializing,
				&unknown,
			},
			hasStatus: true,
			status:    machineidv1pb.BotInstanceHealthStatus_BOT_INSTANCE_HEALTH_STATUS_UNHEALTHY,
		},
		{
			name: "multiple items - unknown",
			services: []*machineidv1pb.BotInstanceServiceHealth{
				&healthy,
				&initializing,
				&unknown,
			},
			hasStatus: true,
			status:    machineidv1pb.BotInstanceHealthStatus_BOT_INSTANCE_HEALTH_STATUS_UNSPECIFIED,
		},
		{
			name: "multiple items - initializing",
			services: []*machineidv1pb.BotInstanceServiceHealth{
				&healthy,
				&initializing,
			},
			hasStatus: true,
			status:    machineidv1pb.BotInstanceHealthStatus_BOT_INSTANCE_HEALTH_STATUS_INITIALIZING,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			has, status := aggregateServiceHealth(tc.services)
			assert.Equal(t, tc.hasStatus, has)
			assert.Equal(t, tc.status, status)
		})
	}
}
func TestListBotInstances(t *testing.T) {
	t.Parallel()

	dynAddr := helpers.NewDynamicServiceAddr(t)
	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.AuthAddr,
			},
		},
	}
	process := makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.Descriptors), withEnableCache(true))
	ctx := t.Context()
	client, err := testenv.NewDefaultAuthClient(process)
	require.NoError(t, err)

	t.Cleanup(func() { _ = client.Close() })

	instance0 := createBotInstance(t, ctx, process)
	instance1 := createBotInstance(t, ctx, process, func(instance *machineidv1pb.BotInstance) {
		instance.Status.InitialHeartbeat.Hostname = "test-hostname-3"
		instance.Status.InitialHeartbeat.Version = "19.0.1"
	})
	instance2 := createBotInstance(t, ctx, process, func(instance *machineidv1pb.BotInstance) {
		instance.Spec.BotName = "test-bot-2"
		instance.Status.InitialHeartbeat.Hostname = "test-hostname-2"
		instance.Status.InitialHeartbeat.Version = "18.1.0"
	})

	// Give the auth cache a chance to catch-up
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		res, _, err := process.GetAuthServer().ListBotInstances(ctx, 0, "", nil)
		require.NoError(t, err)
		require.Len(t, res, 3)
	}, time.Second*10, time.Millisecond*50)

	t.Run("defaults", func(t *testing.T) {
		buf := strings.Builder{}
		cmd := BotsCommand{
			stdout: &buf,
			format: teleport.JSON,
		}

		require.NoError(t, cmd.ListBotInstances(ctx, client))

		res, err := services.UnmarshalProtoResourceArray[*machineidv1pb.BotInstance]([]byte(buf.String()))
		require.NoError(t, err)

		require.Len(t, res, 3)
	})

	t.Run("filter by bot name", func(t *testing.T) {
		buf := strings.Builder{}
		cmd := BotsCommand{
			stdout:  &buf,
			format:  teleport.JSON,
			botName: "test-bot-1",
		}

		require.NoError(t, cmd.ListBotInstances(ctx, client))

		res, err := services.UnmarshalProtoResourceArray[*machineidv1pb.BotInstance]([]byte(buf.String()))
		require.NoError(t, err)

		require.Len(t, res, 2)
		assertContainsInstance(t, res, instance0.GetSpec().GetInstanceId())
		assertContainsInstance(t, res, instance1.GetSpec().GetInstanceId())
	})

	t.Run("filter with search", func(t *testing.T) {
		buf := strings.Builder{}
		cmd := BotsCommand{
			stdout: &buf,
			format: teleport.JSON,
			search: "test-hostname-2",
		}

		require.NoError(t, cmd.ListBotInstances(ctx, client))

		res, err := services.UnmarshalProtoResourceArray[*machineidv1pb.BotInstance]([]byte(buf.String()))
		require.NoError(t, err)

		require.Len(t, res, 1)
		assertContainsInstance(t, res, instance2.GetSpec().GetInstanceId())
	})

	t.Run("filter with query", func(t *testing.T) {
		buf := strings.Builder{}
		cmd := BotsCommand{
			stdout: &buf,
			format: teleport.JSON,
			query:  `status.latest_heartbeat.hostname == "test-hostname-2"`,
		}

		require.NoError(t, cmd.ListBotInstances(ctx, client))

		res, err := services.UnmarshalProtoResourceArray[*machineidv1pb.BotInstance]([]byte(buf.String()))
		require.NoError(t, err)

		require.Len(t, res, 1)
		assertContainsInstance(t, res, instance2.GetSpec().GetInstanceId())
	})

	t.Run("sort by field", func(t *testing.T) {
		buf := strings.Builder{}
		cmd := BotsCommand{
			stdout:    &buf,
			format:    teleport.JSON,
			sortIndex: "version_latest",
		}

		require.NoError(t, cmd.ListBotInstances(ctx, client))

		res, err := services.UnmarshalProtoResourceArray[*machineidv1pb.BotInstance]([]byte(buf.String()))
		require.NoError(t, err)

		require.Len(t, res, 3)
		assert.Equal(t, "18.1.0", res[0].GetStatus().GetInitialHeartbeat().GetVersion())
		assert.Equal(t, "19.0.0", res[1].GetStatus().GetInitialHeartbeat().GetVersion())
		assert.Equal(t, "19.0.1", res[2].GetStatus().GetInitialHeartbeat().GetVersion())
	})

	t.Run("sort order", func(t *testing.T) {
		buf := strings.Builder{}
		cmd := BotsCommand{
			stdout:    &buf,
			format:    teleport.JSON,
			sortIndex: "version_latest",
			sortOrder: "descending",
		}

		require.NoError(t, cmd.ListBotInstances(ctx, client))

		res, err := services.UnmarshalProtoResourceArray[*machineidv1pb.BotInstance]([]byte(buf.String()))
		require.NoError(t, err)

		require.Len(t, res, 3)
		assert.Equal(t, "19.0.1", res[0].GetStatus().GetInitialHeartbeat().GetVersion())
		assert.Equal(t, "19.0.0", res[1].GetStatus().GetInitialHeartbeat().GetVersion())
		assert.Equal(t, "18.1.0", res[2].GetStatus().GetInitialHeartbeat().GetVersion())
	})
}

func assertContainsInstance(t *testing.T, res []*machineidv1pb.BotInstance, instanceId string) {
	assert.True(t, slices.ContainsFunc(res, func(in *machineidv1pb.BotInstance) bool {
		return in.GetSpec().GetInstanceId() == instanceId
	}))
}

func createBotInstance(t *testing.T, ctx context.Context, process *service.TeleportProcess, options ...func(instance *machineidv1pb.BotInstance)) (result *machineidv1pb.BotInstance) {
	heartbeat := &machineidv1pb.BotInstanceStatusHeartbeat{
		RecordedAt: timestamppb.New(time.Now()),
		IsStartup:  true,
		Version:    "19.0.0",
		Hostname:   "test-hostname-1",
		Uptime:     durationpb.New(1 * time.Hour),
		Os:         "linux",
	}

	base := &machineidv1pb.BotInstance{
		Spec: &machineidv1pb.BotInstanceSpec{
			BotName:    "test-bot-1",
			InstanceId: uuid.New().String(),
		},
		Status: &machineidv1pb.BotInstanceStatus{
			InitialHeartbeat: heartbeat,
			LatestHeartbeats: []*machineidv1pb.BotInstanceStatusHeartbeat{
				heartbeat,
			},
		},
	}

	for _, fn := range options {
		fn(base)
	}

	result, err := process.GetAuthServer().CreateBotInstance(ctx, base)
	require.NoError(t, err)

	return
}

func TestListBotInstancesFallback(t *testing.T) {
	t.Parallel()

	dynAddr := helpers.NewDynamicServiceAddr(t)
	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.AuthAddr,
			},
		},
	}
	process := makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.Descriptors), withEnableCache(true))
	ctx := t.Context()
	client, err := testenv.NewDefaultAuthClient(process)
	require.NoError(t, err)

	authClient := &mockBotInstanceListerClient{
		Client: client,
	}

	t.Run("fallback allowed", func(t *testing.T) {
		cmd := BotsCommand{
			stdout: ptr(strings.Builder{}),
			format: teleport.JSON,
		}

		require.NoError(t, cmd.ListBotInstances(ctx, authClient))
	})

	t.Run("fallback not allowed", func(t *testing.T) {
		cmd := BotsCommand{
			stdout: ptr(strings.Builder{}),
			format: teleport.JSON,
			query:  "foo()", // query is only available in ListBotInstancesV2
		}

		err := cmd.ListBotInstances(ctx, authClient)
		require.Error(t, err)
		require.ErrorContains(t, err, "fallback not supported for requests with a query")
	})
}

// mockBotInstanceListerClient is a client which returns NotImplemented for
// ListBotInstancesV2 to simulate a service running an older version.
type mockBotInstanceListerClient struct {
	*authclient.Client
}

func (c *mockBotInstanceListerClient) BotInstanceServiceClient() machineidv1pb.BotInstanceServiceClient {
	return &mockBotInstanceListV2ErrorClient{
		BotInstanceServiceClient: c.Client.BotInstanceServiceClient(),
		errV1:                    nil,
		errV2:                    trace.NotImplemented("not implemeted in mock"),
	}
}

type mockBotInstanceListV2ErrorClient struct {
	machineidv1pb.BotInstanceServiceClient
	errV1 error
	errV2 error
}

func (c *mockBotInstanceListV2ErrorClient) ListBotInstances(ctx context.Context, in *machineidv1pb.ListBotInstancesRequest, opts ...grpc.CallOption) (*machineidv1pb.ListBotInstancesResponse, error) {
	if c.errV1 == nil {
		// Needed for backwards compatibility
		//nolint:staticcheck // SA1019
		return c.BotInstanceServiceClient.ListBotInstances(ctx, in, opts...)
	}
	return nil, c.errV2
}

func (c *mockBotInstanceListV2ErrorClient) ListBotInstancesV2(ctx context.Context, in *machineidv1pb.ListBotInstancesV2Request, opts ...grpc.CallOption) (*machineidv1pb.ListBotInstancesResponse, error) {
	if c.errV2 == nil {
		return c.BotInstanceServiceClient.ListBotInstancesV2(ctx, in, opts...)
	}
	return nil, c.errV2
}

func ptr[T any](v T) *T { return &v }
