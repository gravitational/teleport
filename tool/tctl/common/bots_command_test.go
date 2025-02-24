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

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/config"
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
		tt := tt

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
		tt := tt

		const botName = "test"

		t.Run(tt.desc, func(t *testing.T) {
			mockClient := mockRoleGetterClient{
				roles: tt.knownRoles,
			}

			bot := &machineidv1pb.Bot{
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

			err = cmd.updateBotRoles(context.TODO(), &mockClient, bot, fieldMask)
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
	client := testenv.MakeDefaultAuthClient(t, process)

	tokens, err := client.GetTokens(ctx)
	require.NoError(t, err)
	require.Empty(t, tokens)

	// Create an initial bot
	bot, err := client.BotServiceClient().CreateBot(ctx, &machineidv1pb.CreateBotRequest{
		Bot: &machineidv1pb.Bot{
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
