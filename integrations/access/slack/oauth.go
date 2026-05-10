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

package slack

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/integrations/access/common/auth/storage"
)

const (
	requestTimeout              = 30 * time.Second
	defaultRefreshRetryInterval = 5 * time.Minute
	defaultTokenBufferInterval  = 1 * time.Hour
)

// internal interface used for testing purposes. External consumers must only use Authorizer.
type authorizer interface {
	Refresh(ctx context.Context, refreshToken string) (*storage.Credentials, error)
}

// Authorizer can exchange oauth user tokens against storage.Credentials with Exchange()
// and can refresh existing storage.Credentials with Refresh().
type Authorizer struct {
	client *resty.Client

	clientID     string
	clientSecret string
	log          *slog.Logger
}

func newAuthorizer(client *resty.Client, clientID string, clientSecret string, log *slog.Logger) *Authorizer {
	return &Authorizer{
		client:       client,
		clientID:     clientID,
		clientSecret: clientSecret,
		log:          log,
	}
}

// NewAuthorizer returns a new Authorizer.
//
// clientID is the Client ID for this Slack app as specified by OAuth2.
// clientSecret is the Client Secret for this Slack app as specified by OAuth2.
func NewAuthorizer(clientID string, clientSecret string, log *slog.Logger) *Authorizer {
	client := makeSlackClient(slackAPIURL)
	return newAuthorizer(client, clientID, clientSecret, log.With("authorizer", "slack"))
}

// Exchange implements oauth.Exchanger
func (a *Authorizer) Exchange(ctx context.Context, authorizationCode string, redirectURI string) (*storage.Credentials, error) {
	var result AccessResponse

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	_, err := a.client.R().
		SetContext(ctx).
		SetQueryParam("client_id", a.clientID).
		SetQueryParam("client_secret", a.clientSecret).
		SetQueryParam("code", authorizationCode).
		SetQueryParam("redirect_uri", redirectURI).
		SetResult(&result).
		Post("oauth.v2.access")

	if err != nil {
		a.log.WarnContext(ctx, "Failed to exchange access token.", "error", err)
		return nil, trace.Wrap(err)
	}

	if !result.Ok {
		return nil, trace.Errorf("%s", result.Error)
	}

	return &storage.Credentials{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresAt:    time.Now().UTC().Add(time.Duration(result.ExpiresInSeconds) * time.Second),
	}, nil
}

// Refresh implements oauth.Authorizer
func (a *Authorizer) Refresh(ctx context.Context, refreshToken string) (*storage.Credentials, error) {
	var result AccessResponse

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	_, err := a.client.R().
		SetContext(ctx).
		SetQueryParam("client_id", a.clientID).
		SetQueryParam("client_secret", a.clientSecret).
		SetQueryParam("grant_type", "refresh_token").
		SetQueryParam("refresh_token", refreshToken).
		SetResult(&result).
		Post("oauth.v2.access")

	if err != nil {
		a.log.WarnContext(ctx, "Failed to refresh access token.", "error", err)
		return nil, trace.Wrap(err)
	}

	if !result.Ok {
		return nil, trace.Errorf("%s", result.Error)
	}

	return &storage.Credentials{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresAt:    time.Now().UTC().Add(time.Duration(result.ExpiresInSeconds) * time.Second),
	}, nil
}

// OauthTokenRefresherConfig contains parameters and dependencies for OauthTokenRefresher
type OauthTokenRefresherConfig struct {
	// RetryInterval is the duration the plugin should wait after a refresh failure before retrying.
	// This should be at least 3 times lower than TokenBufferInterval to give the plugin a chance to
	// attempt several renewal before the token expiry.
	RetryInterval time.Duration
	// TokenBufferInterval is the duration before the token expiry at which the plugin should start attempting
	// to renew the token.
	TokenBufferInterval time.Duration

	// InitialCreds are the credentials the oauth refresher is started with.
	// They contain the refresh token that the OauthTokenRefresher will use to
	// perform the refresh.
	InitialCreds storage.Credentials
	// SaveCreds is a function called by the OauthTokenRefresher once the token was refreshed.
	// The function must persist the token into the backend.
	// If the function errors, the OauthTokenRefresher will retry until it succeeds or the
	// service exits.
	SaveCreds SaveCredsFunc
	// Authorizer is the client used to perform the Slack token refresh.
	// This provides the Oauth application credentials.
	Authorizer *Authorizer
	// Clock is used to mock time, mainly in tests.
	Clock clockwork.Clock
	// Log is the refresher logger.
	Log *slog.Logger
}

// CheckAndSetDefaults validates a configuration and sets default values
func (c *OauthTokenRefresherConfig) CheckAndSetDefaults() error {
	if c.RetryInterval == 0 {
		c.RetryInterval = defaultRefreshRetryInterval
	}
	if c.TokenBufferInterval == 0 {
		c.TokenBufferInterval = defaultTokenBufferInterval
	}

	if c.SaveCreds == nil {
		return trace.BadParameter("SaveCreds callback must be set, this is a bug")
	}
	if c.Authorizer == nil {
		return trace.BadParameter("Authorizer must be set, this is a bug")
	}
	if c.InitialCreds.RefreshToken == "" {
		return trace.BadParameter("InitialCreds.RefreshToken must not be empty")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.Log == nil {
		c.Log = slog.Default()
	}
	return nil
}

type SaveCredsFunc func(context.Context, storage.Credentials) error

// OauthTokenRefresher is an implementation of AccessTokenProvider
// that uses OAuth2 refresh token flow to renew the access token.
// The credentials are stored in the given persistent store.
//
// To have an up-to-date token, one must run RefreshLoop() in a background goroutine.
type OauthTokenRefresher struct {
	running             sync.Mutex
	retryInterval       time.Duration
	tokenBufferInterval time.Duration
	saveCreds           SaveCredsFunc
	authorizer          authorizer
	clock               clockwork.Clock

	log *slog.Logger

	lock  sync.Mutex
	creds storage.Credentials
}

// NewOauthTokenRefresher creates a new OauthTokenRefresher from the given config.
// NewOauthTokenRefresher will return an error if the store does not have existing credentials,
// meaning they need to be acquired first (e.g. via OAuth2 authorization code flow).
func NewOauthTokenRefresher(ctx context.Context, cfg OauthTokenRefresherConfig) (*OauthTokenRefresher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	provider := &OauthTokenRefresher{
		retryInterval:       cfg.RetryInterval,
		tokenBufferInterval: cfg.TokenBufferInterval,
		saveCreds:           cfg.SaveCreds,
		authorizer:          cfg.Authorizer,
		clock:               cfg.Clock,
		log:                 cfg.Log,
		creds:               cfg.InitialCreds,
	}

	return provider, nil
}

// GetAccessToken implements AccessTokenProvider()
func (r *OauthTokenRefresher) GetAccessToken() (string, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.creds.AccessToken, nil
}

// RefreshLoop runs the credential refresh process.
func (r *OauthTokenRefresher) RefreshLoop(ctx context.Context) {
	// Don't start a refresh loop if the previous one did not exit yet.
	r.running.Lock()
	defer r.running.Unlock()

	r.lock.Lock()
	interval := r.creds.ExpiresAt.Sub(r.clock.Now()) - r.tokenBufferInterval
	r.lock.Unlock()

	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		Step:   time.Second,
		Max:    time.Minute,
		Jitter: retryutils.DefaultJitter,
		Clock:  r.clock,
	})
	if err != nil {
		r.log.ErrorContext(ctx, "Error while creating the token retry configuration, this is a bug", "error", err)
		return
	}

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
			// Important: we are entering the critical section here.
			// Once we start refreshing the token, we must not stop until we are done writing it to the backend.
			// Failure to do so results in a lost token and broken Slack integration until the user re-registers it.
			// We ignore cancellation here to make sure the refresh process finishes even during a shutdown.
			criticalCtx := context.WithoutCancel(ctx)

			creds, err := r.authorizer.Refresh(criticalCtx, r.creds.RefreshToken)
			if err != nil {
				r.log.ErrorContext(ctx, "Error while refreshing token",
					"error", err,
					"retry_interval", r.retryInterval,
				)
				timer.Reset(r.retryInterval)
			} else {
				r.lock.Lock()
				r.creds = *creds
				r.lock.Unlock()

				var tries int

				err = retry.For(criticalCtx, func() error {
					// We try as long as we can to refresh the token, even if the main context is canceled. But we cannot
					// block the shutdown procedure entirely.
					// So we retry as long as the parent context is still valid. And if it's not we stop after 10 tries.
					select {
					case <-ctx.Done():
						tries++
						if tries > 10 {
							return retryutils.PermanentRetryError(trace.Errorf("Context canceled and failed to persist token"))
						}
					default:
					}
					err := r.saveCreds(criticalCtx, *creds)
					if err != nil {
						// If we land here, we refreshed the Slack token but failed to store it back.
						// This is the worst case scenario: the refresh token is single-use, and we burnt it.
						// This Slack integration will very likely get locked out.
						// The only thing we can do is try again.
						r.log.WarnContext(ctx, "Error while saving credentials to storage", "error", err)
						return err
					}
					return nil
				})
				if err != nil {
					r.log.ErrorContext(ctx, "Failed to persist token and main context is canceled. Aborting. This will break the Slack Oauth refresh cycle.", "tries", tries)
					return
				}

				r.lock.Lock()
				r.creds = *creds
				interval := r.creds.ExpiresAt.Sub(r.clock.Now()) - r.tokenBufferInterval
				r.lock.Unlock()

				r.log.InfoContext(ctx, "Successfully refreshed and persisted credentials, pending plugin restart", "next_refresh", interval)
			}
		}
	}
}
