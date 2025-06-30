/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package accesslist

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
)

type mockMessagingBot struct {
	lastReminderRecipients []common.Recipient
	recipients             map[string]*common.Recipient
	mutex                  sync.Mutex
}

func (m *mockMessagingBot) CheckHealth(ctx context.Context) error {
	return nil
}

func (m *mockMessagingBot) SendReviewReminders(ctx context.Context, recipient common.Recipient, accessLists []*accesslist.AccessList) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.lastReminderRecipients = append(m.lastReminderRecipients, recipient)
	return nil
}

func (m *mockMessagingBot) getLastRecipients() []common.Recipient {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.lastReminderRecipients
}

func (m *mockMessagingBot) resetLastRecipients() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.lastReminderRecipients = make([]common.Recipient, 0)
}

func (m *mockMessagingBot) FetchRecipient(ctx context.Context, recipient string) (*common.Recipient, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	fetchedRecipient, ok := m.recipients[recipient]
	if !ok {
		return nil, trace.NotFound("recipient %s not found", recipient)
	}

	return fetchedRecipient, nil
}

func (m *mockMessagingBot) SupportedApps() []common.App {
	return []common.App{
		NewApp(m),
	}
}

type mockPluginConfig struct {
	client teleport.Client
	bot    *mockMessagingBot
}

func (m *mockPluginConfig) GetTeleportClient(ctx context.Context) (teleport.Client, error) {
	return m.client, nil
}

func (m *mockPluginConfig) GetRecipients() common.RawRecipientsMap {
	return nil
}

func (m *mockPluginConfig) NewBot(clusterName string, webProxyAddr string) (common.MessagingBot, error) {
	return m.bot, nil
}

func (m *mockPluginConfig) GetPluginType() types.PluginType {
	return types.PluginTypeSlack
}

func (m *mockPluginConfig) GetTeleportUser() string {
	return ""
}

func TestAccessListReminders_Single(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClockAt(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC))

	server := newTestAuth(t)

	as := server.Auth()
	t.Cleanup(func() {
		require.NoError(t, as.Close())
	})

	bot := &mockMessagingBot{
		recipients: map[string]*common.Recipient{
			"owner1": {Name: "owner1", ID: "owner1"},
			"owner2": {Name: "owner2", ID: "owner2"},
		},
	}
	app := common.NewApp(&mockPluginConfig{client: as, bot: bot}, "test-plugin")
	app.Clock = clock
	ctx := context.Background()
	go func() {
		app.Run(ctx)
	}()

	ready, err := app.WaitReady(ctx)
	require.NoError(t, err)
	require.True(t, ready)

	t.Cleanup(func() {
		app.Terminate()
		<-app.Done()
		require.NoError(t, app.Err())
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
				Start: oneDay * 14, // Start alerting at two weeks before audit date
			},
		},
	})
	require.NoError(t, err)

	accessLists := []*accesslist.AccessList{accessList}

	// No notifications for today
	advanceAndLookForRecipients(t, bot, as, clock, 0, accessLists)

	// Advance by one week, expect no notifications.
	advanceAndLookForRecipients(t, bot, as, clock, oneDay*7, accessLists)

	// Advance by one week, expect a notification. "not-found" will be missing as a recipient.
	advanceAndLookForRecipients(t, bot, as, clock, oneDay*7, accessLists, "owner1")

	// Add a new owner.
	accessList.Spec.Owners = append(accessList.Spec.Owners, accesslist.Owner{Name: "owner2"})

	// Advance by one day, expect a notification only to the new owner.
	advanceAndLookForRecipients(t, bot, as, clock, oneDay, accessLists, "owner2")

	// Advance by one day, expect no notifications.
	advanceAndLookForRecipients(t, bot, as, clock, oneDay, accessLists)

	// Advance by five more days, to the next week, expect two notifications
	advanceAndLookForRecipients(t, bot, as, clock, oneDay*5, accessLists, "owner1", "owner2")

	// Advance by one day, expect no notifications
	advanceAndLookForRecipients(t, bot, as, clock, oneDay, accessLists)

	// Advance by one day, expect no notifications
	advanceAndLookForRecipients(t, bot, as, clock, oneDay, accessLists)

	// Advance by five more days, to the next week, expect two notifications
	advanceAndLookForRecipients(t, bot, as, clock, oneDay*5, accessLists, "owner1", "owner2")

	// Advance 60 days a day at a time, expect two notifications each time.
	for range 60 {
		// Make sure we only get a notification once per day by iterating through each 6 hours at a time.
		for range 3 {
			advanceAndLookForRecipients(t, bot, as, clock, 6*time.Hour, accessLists)
		}
		advanceAndLookForRecipients(t, bot, as, clock, 6*time.Hour, accessLists, "owner1", "owner2")
	}
}

func TestAccessListReminders_Batched(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
			},
		},
	})

	clock := clockwork.NewFakeClockAt(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC))

	server := newTestAuth(t)

	as := server.Auth()
	t.Cleanup(func() {
		require.NoError(t, as.Close())
	})

	bot := &mockMessagingBot{
		recipients: map[string]*common.Recipient{
			"owner1": {Name: "owner1", ID: "owner1"},
			"owner2": {Name: "owner2", ID: "owner2"},
		},
	}
	app := common.NewApp(&mockPluginConfig{client: as, bot: bot}, "test-plugin")
	app.Clock = clock
	ctx := context.Background()
	go func() {
		app.Run(ctx)
	}()

	ready, err := app.WaitReady(ctx)
	require.NoError(t, err)
	require.True(t, ready)

	t.Cleanup(func() {
		app.Terminate()
		<-app.Done()
		require.NoError(t, app.Err())
	})

	accessList1, err := accesslist.NewAccessList(header.Metadata{
		Name: "test-access-list",
	}, accesslist.Spec{
		Title:  "test access list",
		Owners: []accesslist.Owner{{Name: "owner1"}, {Name: "owner2"}, {Name: "not-found"}},
		Grants: accesslist.Grants{
			Roles: []string{"role"},
		},
		Audit: accesslist.Audit{
			NextAuditDate: clock.Now().Add(28 * 24 * time.Hour), // Four weeks out from today
			Notifications: accesslist.Notifications{
				Start: oneDay * 14, // Start alerting at two weeks before audit date
			},
		},
	})
	require.NoError(t, err)

	accessList2, err := accesslist.NewAccessList(header.Metadata{
		Name: "test-access-list-2",
	}, accesslist.Spec{
		Title:  "test access list 2",
		Owners: []accesslist.Owner{{Name: "owner1"}, {Name: "owner2"}, {Name: "not-found"}},
		Grants: accesslist.Grants{
			Roles: []string{"role"},
		},
		Audit: accesslist.Audit{
			NextAuditDate: clock.Now().Add(28 * 24 * time.Hour), // Four weeks out from today
			Notifications: accesslist.Notifications{
				Start: oneDay * 14, // Start alerting at two weeks before audit date
			},
		},
	})
	require.NoError(t, err)

	accessLists := []*accesslist.AccessList{accessList1, accessList2}

	// No notifications for today
	advanceAndLookForRecipients(t, bot, as, clock, 0, accessLists)

	// Advance by one week, expect no notifications.
	advanceAndLookForRecipients(t, bot, as, clock, oneDay*7, accessLists)

	// Advance by one week, expect a notification. "not-found" will be missing as a recipient.
	advanceAndLookForRecipients(t, bot, as, clock, oneDay*7, accessLists, "owner1", "owner2")

	// Advance another week, expect notifications.
	advanceAndLookForRecipients(t, bot, as, clock, oneDay*7, accessLists, "owner1", "owner2")
}

type mockClient struct {
	mock.Mock
	teleport.Client
}

func (m *mockClient) ListAccessLists(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessList, string, error) {
	args := m.Called(ctx, pageSize, pageToken)
	return args.Get(0).([]*accesslist.AccessList), args.String(1), args.Error(2)
}

func TestAccessListReminders_BadClient(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClockAt(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC))

	server := newTestAuth(t)
	as := server.Auth()
	t.Cleanup(func() {
		require.NoError(t, as.Close())
	})

	// Use this mock client so that we can force ListAccessLists to return an error.
	client := &mockClient{
		Client: as,
	}
	client.On("ListAccessLists", mock.Anything, mock.Anything, mock.Anything).Return(([]*accesslist.AccessList)(nil), "", trace.BadParameter("error"))

	bot := &mockMessagingBot{
		recipients: map[string]*common.Recipient{
			"owner1": {Name: "owner1"},
			"owner2": {Name: "owner2"},
		},
	}
	app := common.NewApp(&mockPluginConfig{client: client, bot: bot}, "test-plugin")
	app.Clock = clock
	ctx := context.Background()
	go func() {
		app.Run(ctx)
	}()

	ready, err := app.WaitReady(ctx)
	require.NoError(t, err)
	require.True(t, ready)

	t.Cleanup(func() {
		app.Terminate()
		<-app.Done()
		require.NoError(t, app.Err())
	})

	clock.BlockUntil(1)
	for i := 1; i <= 6; i++ {
		clock.Advance(3 * time.Hour)
		clock.BlockUntil(1)
		client.AssertNumberOfCalls(t, "ListAccessLists", i)
	}
}

func advanceAndLookForRecipients(t *testing.T,
	bot *mockMessagingBot,
	alSvc services.AccessLists,
	clock *clockwork.FakeClock,
	advance time.Duration,
	accessLists []*accesslist.AccessList,
	recipients ...string) {

	ctx := context.Background()

	for _, accessList := range accessLists {
		_, err := alSvc.UpsertAccessList(ctx, accessList)
		require.NoError(t, err)
	}

	bot.resetLastRecipients()

	var expectedRecipients []common.Recipient
	if len(recipients) > 0 {
		expectedRecipients = make([]common.Recipient, len(recipients))
		for i, r := range recipients {
			expectedRecipients[i] = common.Recipient{Name: r, ID: r}
		}
	}
	clock.Advance(advance)
	clock.BlockUntil(1)

	require.ElementsMatch(t, expectedRecipients, bot.getLastRecipients())
}

func newTestAuth(t *testing.T) *auth.TestServer {
	server, err := auth.NewTestServer(auth.TestServerConfig{
		Auth: auth.TestAuthServerConfig{
			Dir:   t.TempDir(),
			Clock: clockwork.NewFakeClock(),
			AuthPreferenceSpec: &types.AuthPreferenceSpecV2{
				SecondFactor: constants.SecondFactorOn,
				Webauthn: &types.Webauthn{
					RPID: "localhost",
				},
			},
		},
	})
	require.NoError(t, err)
	return server
}
