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

package common

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
	"github.com/gravitational/teleport/integrations/lib"
	pd "github.com/gravitational/teleport/integrations/lib/plugindata"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
)

type mockBot struct {
	lastReminderRecipients    []Recipient
	accessRequestSentMessages SentMessages
	recipients                map[string]*Recipient
}

func (m *mockBot) CheckHealth(ctx context.Context) error {
	return nil
}

func (m *mockBot) AccessListReviewReminder(ctx context.Context, recipients []Recipient, accessList *accesslist.AccessList) error {
	m.lastReminderRecipients = recipients
	return nil
}

func (m *mockBot) BroadcastAccessRequestMessage(ctx context.Context, recipients []Recipient, reqID string, reqData pd.AccessRequestData) (data SentMessages, err error) {
	return m.accessRequestSentMessages, nil
}

func (m *mockBot) PostReviewReply(ctx context.Context, channelID string, threadID string, review types.AccessReview) error {
	return nil
}

func (m *mockBot) UpdateMessages(ctx context.Context, reqID string, data pd.AccessRequestData, messageData SentMessages, reviews []types.AccessReview) error {
	return nil
}

func (m *mockBot) FetchRecipient(ctx context.Context, recipient string) (*Recipient, error) {
	fetchedRecipient, ok := m.recipients[recipient]
	if !ok {
		return nil, trace.NotFound("recipient %s not found", recipient)
	}

	return fetchedRecipient, nil
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

	bot := &mockBot{
		recipients: map[string]*Recipient{
			"owner1": {Name: "owner1"},
			"owner2": {Name: "owner2"},
		},
	}
	app := &BaseApp{
		apiClient:   as,
		accessLists: as,
		bot:         bot,
		clock:       clock,
	}
	app.initBackend()

	app.alMonitorJob = lib.NewServiceJob(app.accessListMonitorRun)

	ctx := context.Background()
	go func() {
		app.Process = lib.NewProcess(ctx)
		app.SpawnCriticalJob(app.alMonitorJob)
	}()

	ready, err := app.alMonitorJob.WaitReady(ctx)
	require.NoError(t, err)
	require.True(t, ready)

	t.Cleanup(func() {
		app.Terminate()
		<-app.alMonitorJob.Done()
		require.NoError(t, app.alMonitorJob.Err())
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
	bot *mockBot,
	alSvc services.AccessLists,
	clock clockwork.FakeClock,
	advance time.Duration,
	accessList *accesslist.AccessList,
	recipients ...string) {

	ctx := context.Background()

	_, err := alSvc.UpsertAccessList(ctx, accessList)
	require.NoError(t, err)

	bot.lastReminderRecipients = nil

	var expectedRecipients []Recipient
	if len(recipients) > 0 {
		expectedRecipients = make([]Recipient, len(recipients))
		for i, recipient := range recipients {
			expectedRecipients[i] = Recipient{Name: recipient}
		}
	}
	clock.Advance(advance)
	//require.NoError(t, app.notifyForAccessListReviews(ctx, accessList))

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		require.ElementsMatch(collect, expectedRecipients, bot.lastReminderRecipients)
	}, 5*time.Second, 250*time.Millisecond)
}
