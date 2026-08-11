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

package azure

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestListSubscriptionIDs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		mockAPI *ARMSubscriptionsMock
		wantIDs []string
	}{
		{
			name: "client lists all subscriptions",
			mockAPI: &ARMSubscriptionsMock{
				Subscriptions: []*armsubscription.Subscription{
					{
						SubscriptionID: to.Ptr("sub1"),
						State:          to.Ptr(armsubscription.SubscriptionStateEnabled),
					},
					{
						SubscriptionID: to.Ptr("sub2"),
						State:          to.Ptr(armsubscription.SubscriptionStateWarned),
					},
					{
						SubscriptionID: to.Ptr("sub3"),
						State:          to.Ptr(armsubscription.SubscriptionStateDeleted),
					},
				},
			},
			wantIDs: []string{"sub1", "sub2"},
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewSubscriptionClient(tt.mockAPI)
			require.NoError(t, err)

			// verify we get all subscriptions
			subIDs, err := client.ListSubscriptionIDs(ctx)
			require.NoError(t, err)
			require.ElementsMatch(t, tt.wantIDs, subIDs)
		})
	}
}

func TestListSubscriptionIDsCache(t *testing.T) {
	t.Parallel()

	t.Run("cached values are cloned and expire", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			api := &ARMSubscriptionsMock{
				Subscriptions: []*armsubscription.Subscription{
					{
						SubscriptionID: new("sub1"),
						State:          new(armsubscription.SubscriptionStateEnabled),
					},
				},
			}
			client, err := NewSubscriptionClient(api)
			require.NoError(t, err)

			ids, err := client.ListSubscriptionIDs(t.Context())
			require.NoError(t, err)
			require.Equal(t, []string{"sub1"}, ids)

			api.Subscriptions = []*armsubscription.Subscription{
				{
					SubscriptionID: new("sub2"),
					State:          new(armsubscription.SubscriptionStateEnabled),
				},
			}
			ids[0] = "modified"

			ids, err = client.ListSubscriptionIDs(t.Context())
			require.NoError(t, err)
			require.Equal(t, []string{"sub1"}, ids)

			time.Sleep(time.Minute + time.Nanosecond)

			ids, err = client.ListSubscriptionIDs(t.Context())
			require.NoError(t, err)
			require.Equal(t, []string{"sub2"}, ids)
		})
	})

	t.Run("errors expire", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			api := &ARMSubscriptionsMock{
				NoAuth: true,
				Subscriptions: []*armsubscription.Subscription{
					{
						SubscriptionID: new("sub1"),
						State:          new(armsubscription.SubscriptionStateEnabled),
					},
				},
			}
			client, err := NewSubscriptionClient(api)
			require.NoError(t, err)

			_, err = client.ListSubscriptionIDs(t.Context())
			require.True(t, trace.IsAccessDenied(err), "got %v", err)

			api.NoAuth = false
			_, err = client.ListSubscriptionIDs(t.Context())
			require.True(t, trace.IsAccessDenied(err), "got %v", err)

			time.Sleep(time.Minute + time.Nanosecond)

			ids, err := client.ListSubscriptionIDs(t.Context())
			require.NoError(t, err)
			require.Equal(t, []string{"sub1"}, ids)
		})
	})
}

func TestExpandSubscriptionIDs(t *testing.T) {
	t.Parallel()

	t.Run("subscriptions without wildcard are unchanged", func(t *testing.T) {
		clients := &subscriptionClientsMock{
			err: errors.New("GetSubscriptionClient should not be called"),
		}
		subscriptions := []string{"sub1", "sub2"}

		ids, err := ExpandSubscriptionIDs(t.Context(), clients, subscriptions)
		require.NoError(t, err)
		require.Equal(t, subscriptions, ids)
		require.Zero(t, clients.getSubscriptionClientCalls)
	})

	t.Run("wildcard is replaced with accessible subscriptions", func(t *testing.T) {
		client, err := NewSubscriptionClient(&ARMSubscriptionsMock{
			Subscriptions: []*armsubscription.Subscription{
				{
					SubscriptionID: new("sub1"),
					State:          new(armsubscription.SubscriptionStateEnabled),
				},
				{
					SubscriptionID: new("sub2"),
					State:          new(armsubscription.SubscriptionStateWarned),
				},
			},
		})
		require.NoError(t, err)
		clients := &subscriptionClientsMock{subscriptionClient: client}

		ids, err := ExpandSubscriptionIDs(t.Context(), clients, []string{"explicit-sub", types.Wildcard})
		require.NoError(t, err)
		require.Equal(t, []string{"sub1", "sub2"}, ids)
		require.Equal(t, 1, clients.getSubscriptionClientCalls)
	})

	t.Run("getting subscription client fails", func(t *testing.T) {
		getClientErr := errors.New("failed to get subscription client")
		clients := &subscriptionClientsMock{err: getClientErr}

		ids, err := ExpandSubscriptionIDs(t.Context(), clients, []string{types.Wildcard})
		require.Nil(t, ids)
		require.ErrorIs(t, err, getClientErr)
	})

	t.Run("listing subscriptions fails", func(t *testing.T) {
		client, err := NewSubscriptionClient(&ARMSubscriptionsMock{NoAuth: true})
		require.NoError(t, err)
		clients := &subscriptionClientsMock{subscriptionClient: client}

		ids, err := ExpandSubscriptionIDs(t.Context(), clients, []string{types.Wildcard})
		require.Nil(t, ids)
		require.True(t, trace.IsAccessDenied(err), "got %v", err)
	})
}

type subscriptionClientsMock struct {
	Clients
	subscriptionClient         *SubscriptionClient
	err                        error
	getSubscriptionClientCalls int
}

func (m *subscriptionClientsMock) GetSubscriptionClient(context.Context) (*SubscriptionClient, error) {
	m.getSubscriptionClientCalls++
	return m.subscriptionClient, m.err
}
