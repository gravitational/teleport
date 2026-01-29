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
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/integrations/access/common/auth/oauth"
	"github.com/gravitational/teleport/integrations/access/common/auth/storage"
)

const defaultRefreshRetryInterval = 5 * time.Minute
const defaultTokenBufferInterval = 1 * time.Hour

// AccessTokenProvider provides a method to get the bearer token
// for use when authorizing to a 3rd-party provider API.
type AccessTokenProvider interface {
	GetAccessToken() (string, error)
}

// StaticAccessTokenProvider is an implementation of AccessTokenProvider
// that always returns the specified token.
type StaticAccessTokenProvider struct {
	token string
}

// NewStaticAccessTokenProvider creates a new StaticAccessTokenProvider.
func NewStaticAccessTokenProvider(token string) *StaticAccessTokenProvider {
	return &StaticAccessTokenProvider{token: token}
}

// GetAccessToken implements AccessTokenProvider
func (s *StaticAccessTokenProvider) GetAccessToken() (string, error) {
	return s.token, nil
}

// RotatedAccessTokenProviderConfig contains parameters and dependencies for RotatedAccessTokenProvider
type RotatedAccessTokenProviderConfig struct {
	RetryInterval       time.Duration
	TokenBufferInterval time.Duration

	Store     storage.Store
	Refresher oauth.Refresher
	Clock     clockwork.Clock

	Log *slog.Logger
}

// CheckAndSetDefaults validates a configuration and sets default values
func (c *RotatedAccessTokenProviderConfig) CheckAndSetDefaults() error {
	if c.RetryInterval == 0 {
		c.RetryInterval = defaultRefreshRetryInterval
	}
	if c.TokenBufferInterval == 0 {
		c.TokenBufferInterval = defaultTokenBufferInterval
	}

	if c.Store == nil {
		return trace.BadParameter("Store must be set")
	}
	if c.Refresher == nil {
		return trace.BadParameter("Refresher must be set")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.Log == nil {
		c.Log = slog.Default()
	}
	return nil
}

// RotatedAccessTokenProvider is an implementation of AccessTokenProvider
// that uses OAuth2 refresh token flow to renew the acess token.
// The credentials are stored in the given persistent store.
//
// To have an up-to-date token, one must run RefreshLoop() in a background goroutine.
type RotatedAccessTokenProvider struct {
	retryInterval       time.Duration
	tokenBufferInterval time.Duration
	store               storage.Store
	refresher           oauth.Refresher
	clock               clockwork.Clock

	log *slog.Logger

	lock  sync.RWMutex // protects the below fields
	creds *storage.Credentials
}

// NewRotatedTokenProvider creates a new RotatedAccessTokenProvider from the given config.
// NewRotatedTokenProvider will return an error if the store does not have existing credentials,
// meaning they need to be acquired first (e.g. via OAuth2 authorization code flow).
func NewRotatedTokenProvider(ctx context.Context, cfg RotatedAccessTokenProviderConfig) (*RotatedAccessTokenProvider, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	provider := &RotatedAccessTokenProvider{
		retryInterval:       cfg.RetryInterval,
		tokenBufferInterval: cfg.TokenBufferInterval,
		store:               cfg.Store,
		refresher:           cfg.Refresher,
		clock:               cfg.Clock,
		log:                 cfg.Log,
	}

	var err error
	provider.creds, err = provider.store.GetCredentials(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return provider, nil
}

// GetAccessToken implements AccessTokenProvider()
func (r *RotatedAccessTokenProvider) GetAccessToken() (string, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return r.creds.AccessToken, nil
}

// RefreshLoop runs the credential refresh process.
func (r *RotatedAccessTokenProvider) RefreshLoop(ctx context.Context) {
	r.lock.RLock()
	interval := r.getRefreshInterval(r.creds)
	r.lock.RUnlock()

	timer := r.clock.NewTimer(interval)
	defer timer.Stop()
	r.log.InfoContext(ctx, "Starting token refresh loop", "next_refresh", interval)

	for {
		select {
		case <-ctx.Done():
			r.log.InfoContext(ctx, "Shutting down")
			return
		case <-timer.Chan():
			r.log.DebugContext(ctx, "Entering token refresh loop")
			creds, err := r.store.GetCredentials(ctx)
			if err != nil {
				r.lock.RLock()
				r.log.WarnContext(ctx, "Error getting credentials, not attempting to refresh credentials", "error", err, "creds_expiry", r.creds.ExpiresAt)
				r.lock.RUnlock()
				// We cannot get the credentials from the backend, something is going on.
				// If we don't have backend access, or we are in an unknown state, we should not attempt to refresh
				// credentials. This will lower the probability of ending up in an awkward state where we refreshed the
				// token but cannot store it.
				timer.Reset(r.retryInterval)
			}

			// Skip if the credentials are sufficiently fresh
			// (in an HA setup another instance might have refreshed the credentials).
			// This is just an optimistic check to potentially reduce API calls.
			// There is no synchronization between several instances of the plugin.
			if creds != nil && !r.shouldRefresh(creds) {
				r.lock.Lock()
				r.creds = creds
				r.lock.Unlock()

				interval := r.getRefreshInterval(creds)
				timer.Reset(interval)
				r.log.DebugContext(ctx, "Existing credentials don't need to be refreshed", "next_refresh", interval)
				r.log.InfoContext(ctx, "Refreshed token", "next_refresh", interval)
				continue
			}

			// Important: we are entering the critical section here.
			// Once we start refreshing the token, we must not stop until we are done writing it to the backend.
			// Failure to do so results in a lost token and broken Slack integration until the user re-registers it.
			// We use a different context here to make sure the refresh process finishes even during a shutdown.
			criticalCtx, cancel := context.WithCancel(context.Background())
			defer cancel()

			creds, err = r.refresh(criticalCtx)
			if err != nil {
				r.log.ErrorContext(ctx, "Error while refreshing token",
					"error", err,
					"retry_interval", r.retryInterval,
				)
				timer.Reset(r.retryInterval)
			} else {
				err := r.store.PutCredentials(criticalCtx, creds)
				if err != nil {
					r.log.ErrorContext(ctx, "Error while storing the refreshed credentials", "error", err)
					// If we land here, we refreshed the Slack token but failed to store it back.
					// This is the worst case scenario: the refresh token is single-use, and we burnt it.
					// This Slack integration will very likely get locked out.
					// The only thing we can do is log the new refresh token, if this happens to be a large-scale issue,
					// this will allow us to perform manual recovery without having to ask every user.
					// It is not ideal to send the token in the logs, but if the integration is still functional and we
					// managed to refresh again in the grace window, the token will become useless in a few seconds.
					r.lock.RLock()
					r.log.ErrorContext(ctx, "Slack integration will get locked out, manual recovery is required",
						"previous_refresh_token", r.creds.RefreshToken,
						"new_refresh_token", creds.RefreshToken,
					)
					r.lock.RUnlock()
					timer.Reset(r.retryInterval)
					continue
				}

				r.lock.Lock()
				r.creds = creds
				r.lock.Unlock()

				interval := r.getRefreshInterval(creds)
				timer.Reset(interval)
				r.log.InfoContext(ctx, "Successfully refreshed credentials", "next_refresh", interval)
			}
		}
	}
}

func (r *RotatedAccessTokenProvider) getRefreshInterval(creds *storage.Credentials) time.Duration {
	d := creds.ExpiresAt.Sub(r.clock.Now()) - r.tokenBufferInterval

	// Timer panics of duration is negative
	if d < 0 {
		d = time.Duration(1)
	}
	return d
}

func (r *RotatedAccessTokenProvider) refresh(ctx context.Context) (*storage.Credentials, error) {
	creds, err := r.refresher.Refresh(ctx, r.creds.RefreshToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return creds, nil
}

func (r *RotatedAccessTokenProvider) shouldRefresh(creds *storage.Credentials) bool {
	now := r.clock.Now()
	refreshAt := creds.ExpiresAt.Add(-r.tokenBufferInterval)
	return now.After(refreshAt) || now.Equal(refreshAt)
}
