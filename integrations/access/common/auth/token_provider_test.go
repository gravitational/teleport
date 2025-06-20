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

package auth

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/integrations/access/common/auth/oauth"
	"github.com/gravitational/teleport/integrations/access/common/auth/storage"
)

type mockRefresher struct {
	refresh func(string) (*storage.Credentials, error)
}

// Refresh implements oauth.Refresher
func (r *mockRefresher) Refresh(ctx context.Context, refreshToken string) (*storage.Credentials, error) {
	return r.refresh(refreshToken)
}

type mockStore struct {
	getCredentials func() (*storage.Credentials, error)
	putCredentials func(*storage.Credentials) error
}

// GetCredentials implements storage.Store
func (s *mockStore) GetCredentials(ctx context.Context) (*storage.Credentials, error) {
	return s.getCredentials()
}

// PutCredentials implements storage.Store
func (s *mockStore) PutCredentials(ctx context.Context, creds *storage.Credentials) error {
	return s.putCredentials(creds)
}

func TestRotatedAccessTokenProvider(t *testing.T) {
	newProvider := func(ctx context.Context, store storage.Store, refresher oauth.Refresher, clock clockwork.Clock, initialCreds *storage.Credentials) *RotatedAccessTokenProvider {
		return &RotatedAccessTokenProvider{
			store:     store,
			refresher: refresher,
			clock:     clock,

			retryInterval:       1 * time.Minute,
			tokenBufferInterval: 1 * time.Hour,

			creds: initialCreds,
			log:   slog.Default(),
		}
	}

	t.Run("Init", func(t *testing.T) {
		clock := clockwork.NewFakeClock()
		initialCreds := &storage.Credentials{
			AccessToken:  "my-access-token",
			RefreshToken: "my-refresh-token",
			ExpiresAt:    clock.Now().Add(2 * time.Hour),
		}

		refresher := &mockRefresher{}
		mockStore := &mockStore{
			getCredentials: func() (*storage.Credentials, error) {
				return initialCreds, nil
			},
		}

		provider, err := NewRotatedTokenProvider(context.Background(), RotatedAccessTokenProviderConfig{
			Store:     mockStore,
			Refresher: refresher,
			Clock:     clock,
		})
		require.NoError(t, err)
		creds, err := provider.GetAccessToken()
		require.NoError(t, err)
		require.Equal(t, initialCreds.AccessToken, creds)
	})

	t.Run("InitFail", func(t *testing.T) {
		clock := clockwork.NewFakeClock()
		refresher := &mockRefresher{}
		mockStore := &mockStore{
			getCredentials: func() (*storage.Credentials, error) {
				return nil, trace.NotFound("not found")
			},
		}

		provider, err := NewRotatedTokenProvider(context.Background(), RotatedAccessTokenProviderConfig{
			Store:     mockStore,
			Refresher: refresher,
			Clock:     clock,
		})
		require.Error(t, err)
		require.Nil(t, provider)
	})

	t.Run("Refresh", func(t *testing.T) {
		clock := clockwork.NewFakeClock()
		initialCreds := &storage.Credentials{
			AccessToken:  "my-access-token",
			RefreshToken: "my-refresh-token",
			ExpiresAt:    clock.Now().Add(2 * time.Hour),
		}
		newCreds := &storage.Credentials{
			AccessToken:  "my-access-token2",
			RefreshToken: "my-refresh-token2",
			ExpiresAt:    clock.Now().Add(4 * time.Hour),
		}

		var storedCreds *storage.Credentials
		var refreshCalled int

		refresher := &mockRefresher{
			refresh: func(refreshToken string) (*storage.Credentials, error) {
				refreshCalled++
				require.Equal(t, refreshToken, initialCreds.RefreshToken)

				// fail the first call
				if refreshCalled == 1 {
					return nil, trace.Errorf("some error")
				}

				return newCreds, nil
			},
		}
		mockStore := &mockStore{
			getCredentials: func() (*storage.Credentials, error) {
				return initialCreds, nil
			},
			putCredentials: func(creds *storage.Credentials) error {
				storedCreds = creds
				return nil
			},
		}

		ctx := t.Context()

		provider := newProvider(ctx, mockStore, refresher, clock, initialCreds)

		go provider.RefreshLoop(ctx)

		clock.BlockUntil(1)
		require.Nil(t, storedCreds) // before attempting refresh

		clock.Advance(1 * time.Hour) // trigger refresh (2 hours - 1 hour buffer)
		clock.BlockUntil(1)
		require.Nil(t, storedCreds) // after first refresh has failed

		clock.Advance(1 * time.Minute) // trigger refresh (after retry period)
		clock.BlockUntil(1)
		require.Equal(t, newCreds, storedCreds)
	})

	t.Run("Cancel", func(t *testing.T) {
		clock := clockwork.NewFakeClock()
		refresher := &mockRefresher{}
		mockStore := &mockStore{}

		initialCreds := &storage.Credentials{
			AccessToken:  "my-access-token",
			RefreshToken: "my-refresh-token",
			ExpiresAt:    clock.Now().Add(2 * time.Hour),
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		provider := newProvider(ctx, mockStore, refresher, clock, initialCreds)
		finished := make(chan struct{}, 1)

		go func() {
			provider.RefreshLoop(ctx)
			finished <- struct{}{}
		}()

		cancel()
		require.Eventually(t, func() bool {
			select {
			case <-finished:
				return true
			default:
				return false
			}
		}, time.Second, time.Second/10)
	})
}
