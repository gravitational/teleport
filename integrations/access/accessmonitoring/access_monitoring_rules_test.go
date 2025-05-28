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

package accessmonitoring

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/lib/services"
)

type mockTeleportClient struct {
	mock.Mock
	teleport.Client
}

func (m *mockTeleportClient) GetUser(ctx context.Context, name string, withSecrets bool) (types.User, error) {
	args := m.Called(ctx, name, withSecrets)
	return args.Get(0).(types.User), args.Error(1)
}

func mockFetchRecipient(ctx context.Context, recipient string) (*common.Recipient, error) {
	return nil, nil
}

func TestRecipeints(t *testing.T) {
	const (
		pluginName = "fakePluginName"
		pluginType = "fakePluginType"
		requester  = "requester@example.com"
		noTraits   = "no-traits@example.com"
		recipient  = "recipient@goteleport.com"
	)

	teleportClient := &mockTeleportClient{}
	teleportClient.
		On("GetUser", mock.Anything, requester, mock.Anything).
		Return(&types.UserV2{
			Spec: types.UserSpecV2{
				Traits: map[string][]string{
					"team": {"example"},
				},
			},
		}, nil)

	teleportClient.
		On("GetUser", mock.Anything, noTraits, mock.Anything).
		Return(&types.UserV2{
			Spec: types.UserSpecV2{
				Traits: map[string][]string{},
			},
		}, nil)

	amrh := NewRuleHandler(RuleHandlerConfig{
		Client:     teleportClient,
		PluginType: pluginType,
		PluginName: pluginName,
		FetchRecipientCallback: func(ctx context.Context, recipient string) (*common.Recipient, error) {
			return emailRecipient(recipient), nil
		},
	})

	rule1, err := services.NewAccessMonitoringRuleWithLabels("rule1", nil, &pb.AccessMonitoringRuleSpec{
		Subjects:  []string{types.KindAccessRequest},
		Condition: `user.traits["team"].contains("example")`,
		Notification: &pb.Notification{
			Name:       pluginName,
			Recipients: []string{recipient},
		},
	})
	require.NoError(t, err)
	amrh.HandleAccessMonitoringRule(context.Background(), types.Event{
		Type:     types.OpPut,
		Resource: types.Resource153ToLegacy(rule1),
	})
	require.Len(t, amrh.getAccessMonitoringRules(), 1)

	ctx := context.Background()

	// Expect recipient from matching rule.
	req := &types.AccessRequestV3{
		Spec: types.AccessRequestSpecV3{
			User: requester,
		},
	}

	recipients := amrh.RecipientsFromAccessMonitoringRules(ctx, req)
	require.ElementsMatch(t, []common.Recipient{*emailRecipient(recipient)}, recipients.ToSlice())

	rawRecipients := amrh.RawRecipientsFromAccessMonitoringRules(ctx, req)
	require.ElementsMatch(t, []string{recipient}, rawRecipients)

	// Expect no recipient from user with no traits.
	req = &types.AccessRequestV3{
		Spec: types.AccessRequestSpecV3{
			User: noTraits,
		},
	}

	recipients = amrh.RecipientsFromAccessMonitoringRules(ctx, req)
	require.ElementsMatch(t, []common.Recipient{}, recipients.ToSlice())

	rawRecipients = amrh.RawRecipientsFromAccessMonitoringRules(ctx, req)
	require.ElementsMatch(t, []string{}, rawRecipients)
}

func emailRecipient(recipient string) *common.Recipient {
	return &common.Recipient{
		Name: recipient,
		ID:   recipient,
		Kind: common.RecipientKindEmail,
	}
}

func TestHandleAccessMonitoringRule(t *testing.T) {
	amrh := NewRuleHandler(RuleHandlerConfig{
		PluginType:             "fakePluginType",
		PluginName:             "fakePluginName",
		FetchRecipientCallback: mockFetchRecipient,
	})

	rule1, err := services.NewAccessMonitoringRuleWithLabels("rule1", nil, &pb.AccessMonitoringRuleSpec{
		Subjects:  []string{types.KindAccessRequest},
		Condition: "true",
		Notification: &pb.Notification{
			Name:       "fakePluginName",
			Recipients: []string{"a", "b"},
		},
	})
	require.NoError(t, err)
	amrh.HandleAccessMonitoringRule(context.Background(), types.Event{
		Type:     types.OpPut,
		Resource: types.Resource153ToLegacy(rule1),
	})
	require.Len(t, amrh.getAccessMonitoringRules(), 1)

	rule2, err := services.NewAccessMonitoringRuleWithLabels("rule2", nil, &pb.AccessMonitoringRuleSpec{
		Subjects:  []string{types.KindAccessRequest},
		Condition: "true",
		Notification: &pb.Notification{
			Name:       "aDifferentFakePlugin",
			Recipients: []string{"a", "b"},
		},
	})
	require.NoError(t, err)
	amrh.HandleAccessMonitoringRule(context.Background(), types.Event{
		Type:     types.OpPut,
		Resource: types.Resource153ToLegacy(rule2),
	})
	require.Len(t, amrh.getAccessMonitoringRules(), 1)

	amrh.HandleAccessMonitoringRule(context.Background(), types.Event{
		Type:     types.OpDelete,
		Resource: types.Resource153ToLegacy(rule1),
	})
	require.Empty(t, amrh.getAccessMonitoringRules())
}

func TestHandleAccessMonitoringRulePluginNameMisMatch(t *testing.T) {
	amrh := NewRuleHandler(RuleHandlerConfig{
		PluginName:             "fakePluginName",
		FetchRecipientCallback: mockFetchRecipient,
	})

	rule1, err := services.NewAccessMonitoringRuleWithLabels("rule1", nil, &pb.AccessMonitoringRuleSpec{
		Subjects:  []string{types.KindAccessRequest},
		Condition: "true",
		Notification: &pb.Notification{
			Name:       "notTheFakePluginName",
			Recipients: []string{"a", "b"},
		},
	})
	require.NoError(t, err)
	amrh.HandleAccessMonitoringRule(context.Background(), types.Event{
		Type:     types.OpPut,
		Resource: types.Resource153ToLegacy(rule1),
	})
	require.Empty(t, amrh.getAccessMonitoringRules())

	rule2, err := services.NewAccessMonitoringRuleWithLabels("rule2", nil, &pb.AccessMonitoringRuleSpec{
		Subjects:  []string{types.KindAccessRequest},
		Condition: "true",
		Notification: &pb.Notification{
			Name:       "fakePluginName",
			Recipients: []string{"c", "d"},
		},
	})
	require.NoError(t, err)
	amrh.HandleAccessMonitoringRule(context.Background(), types.Event{
		Type:     types.OpPut,
		Resource: types.Resource153ToLegacy(rule2),
	})
	require.Len(t, amrh.getAccessMonitoringRules(), 1)

	amrh.HandleAccessMonitoringRule(context.Background(), types.Event{
		Type:     types.OpDelete,
		Resource: types.Resource153ToLegacy(rule2),
	})
	require.Empty(t, amrh.getAccessMonitoringRules())
}
