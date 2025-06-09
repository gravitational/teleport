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

package credprovider

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/utils/testutils"
)

type fakeSTSClient struct {
	clock clockwork.Clock
	err   error
	sync.Mutex
}

func (f *fakeSTSClient) setError(err error) {
	f.Lock()
	f.err = err
	f.Unlock()
}

func (f *fakeSTSClient) getError() error {
	f.Lock()
	defer f.Unlock()
	return f.err
}

func (f *fakeSTSClient) AssumeRoleWithWebIdentity(ctx context.Context, params *sts.AssumeRoleWithWebIdentityInput, optFns ...func(*sts.Options)) (*sts.AssumeRoleWithWebIdentityOutput, error) {
	if err := f.getError(); err != nil {
		return nil, err
	}

	expiration := f.clock.Now().Add(time.Second * time.Duration(*params.DurationSeconds))
	return &sts.AssumeRoleWithWebIdentityOutput{
		Credentials: &ststypes.Credentials{
			Expiration: &expiration,
			// These are example values taken from https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRoleWithWebIdentity.html
			SessionToken:    aws.String("AQoDYXdzEE0a8ANXXXXXXXXNO1ewxE5TijQyp+IEXAMPLE"),
			SecretAccessKey: aws.String("wJalrXUtnFEMI/K7MDENG/bPxRfiCYzEXAMPLEKEY"),
			AccessKeyId:     aws.String("ASgeIAIOSFODNN7EXAMPLE"),
		},
	}, nil
}

func TestCredentialsCache(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			Cloud: true,
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.ExternalAuditStorage: {Enabled: true},
			},
		},
	})

	// GIVEN a configured and running credential cache...
	clock := clockwork.NewFakeClock()
	stsClient := &fakeSTSClient{
		clock: clock,
	}
	cacheUnderTest, err := NewCredentialsCache(CredentialsCacheOptions{
		STSClient:   stsClient,
		Integration: "test",
		Clock:       clock,
		GenerateOIDCTokenFn: func(ctx context.Context, integration string) (string, error) {
			return uuid.NewString(), nil
		},
	})
	require.NoError(t, err)
	require.NotNil(t, cacheUnderTest)
	go cacheUnderTest.Run(ctx)

	advanceClock := func(d time.Duration) {
		// Wait for the run loop to actually wait on the clock ticker before advancing. If we advance before
		// the loop waits on the ticker, it may never tick.
		clock.BlockUntil(1)
		clock.Advance(d)
	}

	checkRetrieveCredentials := func(t require.TestingT, expectErr error) {
		_, err := cacheUnderTest.Retrieve(ctx)
		assert.ErrorIs(t, err, expectErr)
	}

	checkRetrieveCredentialsWithExpiry := func(t require.TestingT, expectExpiry time.Time) {
		creds, err := cacheUnderTest.Retrieve(ctx)
		assert.NoError(t, err)
		if err == nil {
			assert.WithinDuration(t, expectExpiry, creds.Expires, time.Minute)
		}
	}

	const (
		// Using a longer wait time to avoid test flakes observed with 1s wait.
		waitFor = 10 * time.Second
		// We're using a short sleep (1ms) to allow the refresh loop goroutine to get scheduled.
		// This keeps the test fast under normal conditions. If there's CPU starvation in CI,
		// neither the test goroutine nor the refresh loop are likely getting scheduled often,
		// so this shouldn't result in a busy loop.
		tick = 1 * time.Millisecond
	)

	t.Run("Retrieve", func(t *testing.T) {
		// Assert that credentials can be retrieved when everything is happy.
		// EventuallyWithT is necessary to allow credentialsCache.run to be
		// scheduled after SetGenerateOIDCTokenFn above.
		initialCredentialExpiry := clock.Now().Add(TokenLifetime)
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			checkRetrieveCredentialsWithExpiry(t, initialCredentialExpiry)
		}, waitFor, tick)
	})

	t.Run("CachedCredsArePreservedOnError", func(t *testing.T) {
		initialCredentialExpiry := clock.Now().Add(TokenLifetime)
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			checkRetrieveCredentialsWithExpiry(t, initialCredentialExpiry)
		}, waitFor, tick)

		// Assert that the good cached credentials are still used even if sts starts
		// returning errors.
		stsError := errors.New("test error")
		stsClient.setError(stsError)
		// Test immediately
		checkRetrieveCredentialsWithExpiry(t, initialCredentialExpiry)
		// Advance to 1 minute before first refresh attempt
		advanceClock(TokenLifetime - refreshBeforeExpirationPeriod - time.Minute)
		checkRetrieveCredentialsWithExpiry(t, initialCredentialExpiry)
		// Advance to 1 minute after first refresh attempt
		advanceClock(2 * time.Minute)
		checkRetrieveCredentialsWithExpiry(t, initialCredentialExpiry)
		// Advance to 1 minute before credential expiry
		advanceClock(refreshBeforeExpirationPeriod - 2*time.Minute)
		checkRetrieveCredentialsWithExpiry(t, initialCredentialExpiry)

		// Advance 1 minute past the credential expiry and make sure we get the
		// expected error.
		advanceClock(2 * time.Minute)
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			checkRetrieveCredentials(t, stsError)
		}, waitFor, tick)

		// Fix STS and make sure we stop getting errors within refreshCheckInterval
		stsClient.setError(nil)
		advanceClock(refreshCheckInterval)
		newCredentialExpiry := clock.Now().Add(TokenLifetime)
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			checkRetrieveCredentialsWithExpiry(t, newCredentialExpiry)
		}, waitFor, tick)
	})

	t.Run("WindowedErrors", func(t *testing.T) {
		// Test a scenario where STS is returning errors in two different 10-minute windows: the first surrounding
		// the expected cert refresh time, and the second surrounding the cert expiry time.
		// In this case the credentials cache should refresh the certs somewhere between those two outages, and
		// clients should never see an error retrieving credentials.
		newCredentialExpiry := clock.Now().Add(TokenLifetime)
		expectedRefreshTime := newCredentialExpiry.Add(-refreshBeforeExpirationPeriod)
		credentialsUpdated := false
		done := newCredentialExpiry.Add(10 * time.Minute)
		stsError := errors.New("test error")
		for clock.Now().Before(done) {
			if clock.Now().Sub(expectedRefreshTime).Abs() < 5*time.Minute ||
				clock.Now().Sub(newCredentialExpiry).Abs() < 5*time.Minute {
				// Within one of the 10-minute outage windows, make the STS client return errors.
				stsClient.setError(stsError)
				advanceClock(time.Minute)
			} else {
				// Not within an outage window, STS client should not return errors.
				stsClient.setError(nil)
				advanceClock(time.Minute)

				if !credentialsUpdated && clock.Now().After(expectedRefreshTime) {
					// This is after the expected refresh time and not within an outage window, for the test to
					// not be flaky we need to wait for the cache run loop to get a chance to refresh the
					// credentials.
					expectedExpiry := clock.Now().Add(TokenLifetime)
					require.EventuallyWithT(t, func(t *assert.CollectT) {
						creds, err := cacheUnderTest.Retrieve(ctx)
						assert.NoError(t, err)
						assert.WithinDuration(t, expectedExpiry, creds.Expires, 2*time.Minute)
					}, waitFor, tick)
					credentialsUpdated = true
				}
			}

			// Assert that there is never an error getting credentials.
			checkRetrieveCredentials(t, nil)
		}
	})
}

func TestCredentialsCacheRetrieveBeforeInit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clock := clockwork.NewFakeClock()
	stsClient := &fakeSTSClient{
		clock: clock,
	}
	cache, err := NewCredentialsCache(CredentialsCacheOptions{
		STSClient:               stsClient,
		Integration:             "test",
		Clock:                   clock,
		AllowRetrieveBeforeInit: true,
	})
	require.NoError(t, err)

	testutils.RunTestBackgroundTask(ctx, t, &testutils.TestBackgroundTask{
		Name: "cache.Run",
		Task: func(ctx context.Context) error {
			cache.Run(ctx)
			return nil
		},
		Terminate: func() error {
			cancel()
			return nil
		},
	})

	// cache.Retrieve should return immediately with errNotReady if
	// SetGenerateOIDCTokenFn has not been called yet.
	_, err = cache.Retrieve(ctx)
	require.ErrorIs(t, err, errNotReady)

	// The GenerateOIDCTokenFn can be set after the cache has been initialized.
	cache.SetGenerateOIDCTokenFn(func(ctx context.Context, integration string) (string, error) {
		return uuid.NewString(), nil
	})
	// WaitForFirstCredsOrErr should usually be called after
	// SetGenerateOIDCTokenFn to make sure credentials are ready before they
	// will be relied upon.
	cache.WaitForFirstCredsOrErr(ctx)
	// Now cache.Retrieve should not return an error.
	creds, err := cache.Retrieve(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, creds.SecretAccessKey)
}
