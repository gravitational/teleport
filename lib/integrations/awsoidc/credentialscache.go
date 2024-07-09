// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package awsoidc

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

const (
	// TokenLifetime is the lifetime of OIDC tokens used by the
	// ExternalAuditStorage service with the AWS OIDC integration.
	TokenLifetime = time.Hour

	refreshBeforeExpirationPeriod = 15 * time.Minute
	refreshCheckInterval          = 30 * time.Second
	retrieveTimeout               = 30 * time.Second
)

// GenerateOIDCTokenFn is a function that should return a valid, signed JWT for
// authenticating to AWS via OIDC.
type GenerateOIDCTokenFn func(ctx context.Context, integration string) (string, error)

type credsOrErr struct {
	creds aws.Credentials
	err   error
}

// CredentialsCache is used to store and refresh AWS credentials used with
// AWS OIDC integration.
//
// Credentials are valid for 1h, but they cannot be refreshed if Proxy is down,
// so we attempt to refresh the credentials early and retry on failure.
type CredentialsCache struct {
	log *slog.Logger

	roleARN     arn.ARN
	integration string

	// generateOIDCTokenFn is dynamically set after auth is initialized.
	generateOIDCTokenFn GenerateOIDCTokenFn

	// initialized communicates (via closing channel) that generateOIDCTokenFn is set.
	initialized      chan struct{}
	closeInitialized func()

	// gotFirstCredsOrErr communicates (via closing channel) that the first
	// credsOrErr has been set.
	gotFirstCredsOrErr      chan struct{}
	closeGotFirstCredsOrErr func()

	credsOrErr   credsOrErr
	credsOrErrMu sync.RWMutex

	stsClient stscreds.AssumeRoleWithWebIdentityAPIClient
	clock     clockwork.Clock
}

type CredentialsCacheOptions struct {
	Log       *slog.Logger
	Clock     clockwork.Clock
	STSClient stscreds.AssumeRoleWithWebIdentityAPIClient
}

func (opts *CredentialsCacheOptions) CheckAndSetDefaults() error {
	if opts.STSClient == nil {
		return trace.BadParameter("stsClient must be provided")
	}

	if opts.Log == nil {
		opts.Log = slog.Default().With(teleport.ComponentKey, "AWS-OIDC-CredentialCache")
	}

	if opts.Clock == nil {
		opts.Clock = clockwork.NewRealClock()
	}

	return nil
}

var errNotReady error = errors.New("ExternalAuditStorage: credential cache not yet initialized")

func NewCredentialsCache(integration string, roleARN arn.ARN, options *CredentialsCacheOptions) (*CredentialsCache, error) {

	if err := options.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err, "creating credentials cache")
	}

	initialized := make(chan struct{})
	gotFirstCredsOrErr := make(chan struct{})

	return &CredentialsCache{
		roleARN:                 roleARN,
		integration:             integration,
		log:                     options.Log,
		initialized:             initialized,
		closeInitialized:        sync.OnceFunc(func() { close(initialized) }),
		gotFirstCredsOrErr:      gotFirstCredsOrErr,
		closeGotFirstCredsOrErr: sync.OnceFunc(func() { close(gotFirstCredsOrErr) }),
		credsOrErr:              credsOrErr{err: errNotReady},
		clock:                   options.Clock,
		stsClient:               options.STSClient,
	}, nil
}

func (cc *CredentialsCache) SetGenerateOIDCTokenFn(fn GenerateOIDCTokenFn) {
	cc.generateOIDCTokenFn = fn
	cc.closeInitialized()
}

// Retrieve implements [aws.CredentialsProvider] and returns the latest cached
// credentials, or an error if no credentials have been generated yet or the
// last generated credentials have expired.
func (cc *CredentialsCache) Retrieve(ctx context.Context) (aws.Credentials, error) {
	cc.credsOrErrMu.RLock()
	defer cc.credsOrErrMu.RUnlock()

	if cc.credsOrErr.err != nil {
		cc.log.WarnContext(ctx, "Returning error to AWS client", errorValue(cc.credsOrErr.err))
	}

	return cc.credsOrErr.creds, cc.credsOrErr.err
}

func (cc *CredentialsCache) Run(ctx context.Context) {
	// Wait for initialized signal before running loop.
	select {
	case <-cc.initialized:
	case <-ctx.Done():
		cc.log.DebugContext(ctx, "Context canceled before initialized.")
		return
	}

	cc.refreshIfNeeded(ctx)

	ticker := cc.clock.NewTicker(refreshCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.Chan():
			cc.refreshIfNeeded(ctx)
		case <-ctx.Done():
			cc.log.DebugContext(ctx, "Context canceled, stopping refresh loop.")
			return
		}
	}
}

func (cc *CredentialsCache) refreshIfNeeded(ctx context.Context) {
	cc.log.DebugContext(ctx, "Entering credentialCache.refreshIfNeeded()")
	defer cc.log.DebugContext(ctx, "Exiting credentialCache.refreshIfNeeded()")

	credsFromCache, err := cc.Retrieve(ctx)
	if err == nil &&
		credsFromCache.HasKeys() &&
		cc.clock.Now().Add(refreshBeforeExpirationPeriod).Before(credsFromCache.Expires) {
		// No need to refresh, credentials in cache are still valid for longer
		// than refreshBeforeExpirationPeriod
		return
	}
	cc.log.DebugContext(ctx, "Refreshing credentials.")

	creds, err := cc.refresh(ctx)
	if err != nil {
		cc.log.WarnContext(ctx, "Failed to retrieve new credentials", errorValue(err))
		now := cc.clock.Now()
		// If we were not able to refresh, check if existing credentials in
		// cache are still valid. If yes, just log debug, it will be retried on
		// next interval check.
		if credsFromCache.HasKeys() && now.Before(credsFromCache.Expires) {
			cc.log.DebugContext(ctx, "Using existing credentials",
				"ttl", ttlValue{now: now, expiry: credsFromCache.Expires})
			return
		}
		// If existing creds are expired, update cached error.
		cc.log.ErrorContext(ctx, "Setting cached error", "error", err)
		cc.setCredsOrErr(credsOrErr{err: trace.Wrap(err)})
		return
	}

	// Refresh went well, update cached creds.
	cc.setCredsOrErr(credsOrErr{creds: creds})
	cc.log.Debug("Successfully refreshed credentials",
		slog.Time("expires", creds.Expires))
}

type ttlValue struct {
	now    time.Time
	expiry time.Time
}

func (v *ttlValue) LogValue() slog.Value {
	return slog.DurationValue(v.expiry.Sub(v.now).Round(time.Second))
}

func (cc *CredentialsCache) setCredsOrErr(coe credsOrErr) {
	cc.credsOrErrMu.Lock()
	defer cc.credsOrErrMu.Unlock()
	cc.credsOrErr = coe
	cc.closeGotFirstCredsOrErr()
}

func (cc *CredentialsCache) refresh(ctx context.Context) (aws.Credentials, error) {
	cc.log.InfoContext(ctx, "Refreshing AWS credentials")
	defer cc.log.InfoContext(ctx, "Exiting AWS credentials refresh")

	cc.log.InfoContext(ctx, "Generating Token", "integration", cc.integration)
	oidcToken, err := cc.generateOIDCTokenFn(ctx, cc.integration)
	if err != nil {
		cc.log.ErrorContext(ctx, "Token generation failed", errorValue(err))
		return aws.Credentials{}, trace.Wrap(err)
	}

	roleProvider := stscreds.NewWebIdentityRoleProvider(
		cc.stsClient,
		cc.roleARN.String(),
		identityToken(oidcToken),
		func(wiro *stscreds.WebIdentityRoleOptions) {
			wiro.Duration = TokenLifetime
		},
	)

	ctx, cancel := context.WithTimeout(ctx, retrieveTimeout)
	defer cancel()

	cc.log.InfoContext(ctx, "Retrieving AWS role credentials")

	creds, err := roleProvider.Retrieve(ctx)
	if err != nil {
		cc.log.ErrorContext(ctx, "Role retrieval failed", errorValue(err))
	}

	return creds, trace.Wrap(err)
}

func (cc *CredentialsCache) WaitForFirstCredsOrErr(ctx context.Context) {
	cc.log.InfoContext(ctx, "Entering wait on first credential refresh")
	defer cc.log.InfoContext(ctx, "Exiting wait on first credential refresh")

	select {
	case <-ctx.Done():
	case <-cc.gotFirstCredsOrErr:
	}
}

// identityToken is an implementation of [stscreds.IdentityTokenRetriever] for returning a static token.
type identityToken string

// GetIdentityToken returns the token configured.
func (j identityToken) GetIdentityToken() ([]byte, error) {
	return []byte(j), nil
}

func errorValue(v error) slog.Attr {
	return slog.Any("error", v)
}
