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

package accesslist

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/recipient"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
)

type mockMessagingBot struct {
	lastReminderRecipients []recipient.Recipient
	recipients             map[string]*recipient.Recipient
}

func (m *mockMessagingBot) CheckHealth(ctx context.Context) error {
	return nil
}

func (m *mockMessagingBot) SendReviewReminders(ctx context.Context, recipients []recipient.Recipient, accessList *accesslist.AccessList) error {
	m.lastReminderRecipients = recipients
	return nil
}

func (m *mockMessagingBot) FetchRecipient(ctx context.Context, recipient string) (*recipient.Recipient, error) {
	fetchedRecipient, ok := m.recipients[recipient]
	if !ok {
		return nil, trace.NotFound("recipient %s not found", recipient)
	}

	return fetchedRecipient, nil
}

type mockPluginConfig struct {
	as  *auth.Server
	bot *mockMessagingBot
}

func (m *mockPluginConfig) GetTeleportClient(ctx context.Context) (teleport.Client, error) {
	return m.as, nil
}

func (m *mockPluginConfig) GetRecipients() recipient.RawRecipientsMap {
	return nil
}

func (m *mockPluginConfig) NewBot(clusterName string, webProxyAddr string) (common.MessagingBot, error) {
	return m.bot, nil
}

func (m *mockPluginConfig) GetPluginType() types.PluginType {
	return types.PluginTypeSlack
}

func TestAccessListReminders(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClockAt(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC))

	server, err := auth.NewTestServer(auth.TestServerConfig{
		Auth: auth.TestAuthServerConfig{
			Dir:   t.TempDir(),
			Clock: clock,
		},
	})
	require.NoError(t, err)
	as := server.Auth()

	bot := &mockMessagingBot{
		recipients: map[string]*recipient.Recipient{
			"owner1": {Name: "owner1"},
			"owner2": {Name: "owner2"},
		},
	}
	app := NewApp()
	app.clock = clock
	baseApp := common.NewApp(&mockPluginConfig{as: as, bot: bot}, "test-plugin").
		AddApp(app)
	ctx := context.Background()
	go func() {
		require.NoError(t, baseApp.Run(ctx))
	}()

	ready, err := app.job.WaitReady(ctx)
	require.NoError(t, err)
	require.True(t, ready)

	t.Cleanup(func() {
		baseApp.Terminate()
		<-app.job.Done()
		require.NoError(t, app.job.Err())
	})

	accessList, err := accesslist.NewAccessList(header.Metadata{
		Name: "test-access-list",
	}, accesslist.Spec{
		Title:  "test access list",
		Owners: []accesslist.Owner{{Name: "owner1"}, {Name: "not-found"}},
		Grants: accesslist.Grants{
			Roles: []string{"role"},
		},
		Audit: accesslist.Audit{
			NextAuditDate: clock.Now().Add(28 * 24 * time.Hour), // Four weeks out from today
			Notifications: accesslist.Notifications{
				Start: time.Hour * 24 * 14, // Start alerting at two weeks before audit date
			},
		},
	})
	require.NoError(t, err)

	// No notifications for today
	advanceAndLookForRecipients(t, bot, as, clock, 0, accessList)

	// Advance by one week, expect no notifications.
	advanceAndLookForRecipients(t, bot, as, clock, 24*7*time.Hour, accessList)

	// Advance by one week, expect a notification. "not-found" will be missing as a recipient.
	advanceAndLookForRecipients(t, bot, as, clock, 24*7*time.Hour, accessList, "owner1")

	// Add a new owner.
	accessList.Spec.Owners = append(accessList.Spec.Owners, accesslist.Owner{Name: "owner2"})

	// Advance by one day, expect a notification only to the new owner.
	advanceAndLookForRecipients(t, bot, as, clock, 24*time.Hour, accessList, "owner2")

	// Advance by one day, expect no notifications.
	advanceAndLookForRecipients(t, bot, as, clock, 24*time.Hour, accessList)

	// Advance by five more days, to the next week, expect two notifications
	advanceAndLookForRecipients(t, bot, as, clock, 24*5*time.Hour, accessList, "owner1", "owner2")

	// Advance by one day, expect no notifications
	advanceAndLookForRecipients(t, bot, as, clock, 24*time.Hour, accessList)

	// Advance by one day, expect no notifications
	advanceAndLookForRecipients(t, bot, as, clock, 24*time.Hour, accessList)

	// Advance by five more days, to the next week, expect two notifications
	advanceAndLookForRecipients(t, bot, as, clock, 24*5*time.Hour, accessList, "owner1", "owner2")

	// Advance by one year a week at a time, expect two notifications each time.
	for i := 0; i < 52; i++ {
		advanceAndLookForRecipients(t, bot, as, clock, 24*7*time.Hour, accessList, "owner1", "owner2")
	}
}

func advanceAndLookForRecipients(t *testing.T,
	bot *mockMessagingBot,
	alSvc services.AccessLists,
	clock clockwork.FakeClock,
	advance time.Duration,
	accessList *accesslist.AccessList,
	recipients ...string) {
	t.Helper()

	ctx := context.Background()

	_, err := alSvc.UpsertAccessList(ctx, accessList)
	require.NoError(t, err)

	bot.lastReminderRecipients = nil

	var expectedRecipients []recipient.Recipient
	if len(recipients) > 0 {
		expectedRecipients = make([]recipient.Recipient, len(recipients))
		for i, r := range recipients {
			expectedRecipients[i] = recipient.Recipient{Name: r}
		}
	}
	clock.Advance(advance)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		assert.ElementsMatch(t, expectedRecipients, bot.lastReminderRecipients)
	}, 5*time.Second, 250*time.Millisecond)
}
