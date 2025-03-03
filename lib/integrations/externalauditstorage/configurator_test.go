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

package externalauditstorage

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/externalauditstorage"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services/local"
)

func testOIDCIntegration(t *testing.T) *types.IntegrationV1 {
	oidcIntegration, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: "aws-integration-1"},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN: "arn:aws:iam::account:role/role1",
		},
	)
	require.NoError(t, err)
	return oidcIntegration
}

func testDraftExternalAuditStorage(t *testing.T) *externalauditstorage.ExternalAuditStorage {
	draft, err := externalauditstorage.NewDraftExternalAuditStorage(header.Metadata{}, externalauditstorage.ExternalAuditStorageSpec{
		IntegrationName:        "aws-integration-1",
		PolicyName:             "ecaPolicy",
		Region:                 "us-west-2",
		SessionRecordingsURI:   "s3://bucket/sess_rec",
		AthenaWorkgroup:        "primary",
		GlueDatabase:           "teleport_db",
		GlueTable:              "teleport_table",
		AuditEventsLongTermURI: "s3://bucket/events",
		AthenaResultsURI:       "s3://bucket/results",
	})
	require.NoError(t, err)
	return draft
}

func TestConfiguratorIsUsed(t *testing.T) {
	ctx := context.Background()

	draftConfig := testDraftExternalAuditStorage(t)
	tests := []struct {
		name              string
		modules           *modules.TestModules
		resourceServiceFn func(t *testing.T, s *local.ExternalAuditStorageService)
		wantIsUsed        bool
	}{
		{
			name: "not cloud",
			modules: &modules.TestModules{
				TestFeatures: modules.Features{
					Cloud: false,
				},
			},
			wantIsUsed: false,
		},
		{
			name: "cloud team",
			modules: &modules.TestModules{
				TestFeatures: modules.Features{
					Cloud:               true,
					IsUsageBasedBilling: true,
				},
			},
			wantIsUsed: false,
		},
		{
			name: "cloud enterprise without config",
			modules: &modules.TestModules{
				TestFeatures: modules.Features{
					Cloud: true,
					Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
						entitlements.ExternalAuditStorage: {Enabled: true},
					},
				},
			},
			wantIsUsed: false,
		},
		{
			name: "cloud enterprise with only draft",
			modules: &modules.TestModules{
				TestFeatures: modules.Features{
					Cloud: true,
					Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
						entitlements.ExternalAuditStorage: {Enabled: true},
					},
				},
			},
			// Just create draft, External Audit Storage should be disabled, it's
			// active only when the draft is promoted to cluster external audit
			// storage resource.
			resourceServiceFn: func(t *testing.T, s *local.ExternalAuditStorageService) {
				_, err := s.UpsertDraftExternalAuditStorage(ctx, draftConfig)
				require.NoError(t, err)
			},
			wantIsUsed: false,
		},
		{
			name: "cloud enterprise with cluster config",
			modules: &modules.TestModules{
				TestFeatures: modules.Features{
					Cloud: true,
					Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
						entitlements.ExternalAuditStorage: {Enabled: true},
					},
				},
			},
			// Create draft and promote it to cluster.
			resourceServiceFn: func(t *testing.T, s *local.ExternalAuditStorageService) {
				_, err := s.UpsertDraftExternalAuditStorage(ctx, draftConfig)
				require.NoError(t, err)
				err = s.PromoteToClusterExternalAuditStorage(ctx)
				require.NoError(t, err)
			},
			wantIsUsed: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mem, err := memory.New(memory.Config{})
			require.NoError(t, err)

			integrationSvc, err := local.NewIntegrationsService(mem)
			require.NoError(t, err)
			_, err = integrationSvc.CreateIntegration(ctx, testOIDCIntegration(t))
			require.NoError(t, err)

			ecaSvc := local.NewExternalAuditStorageService(mem)
			if tt.resourceServiceFn != nil {
				tt.resourceServiceFn(t, ecaSvc)
			}

			modules.SetTestModules(t, tt.modules)

			c, err := NewConfigurator(ctx, ecaSvc, integrationSvc, nil /*alertService*/)
			require.NoError(t, err)
			require.Equal(t, tt.wantIsUsed, c.IsUsed(),
				"Configurator.IsUsed() = %v, want %v", c.IsUsed(), tt.wantIsUsed)
			if c.IsUsed() {
				require.Equal(t, draftConfig.Spec, *c.GetSpec())
			}
		})
	}
}

func TestCredentialsCache(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			Cloud: true,
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.ExternalAuditStorage: {Enabled: true},
			},
		},
	})

	mem, err := memory.New(memory.Config{})
	require.NoError(t, err)

	// Pre-req: existing AWS OIDC integration
	integrationSvc, err := local.NewIntegrationsService(mem)
	require.NoError(t, err)
	oidcIntegration := testOIDCIntegration(t)
	_, err = integrationSvc.CreateIntegration(ctx, oidcIntegration)
	require.NoError(t, err)

	// Pre-req: existing cluster ExternalAuditStorage configuration
	draftConfig := testDraftExternalAuditStorage(t)
	svc := local.NewExternalAuditStorageService(mem)
	_, err = svc.UpsertDraftExternalAuditStorage(ctx, draftConfig)
	require.NoError(t, err)
	err = svc.PromoteToClusterExternalAuditStorage(ctx)
	require.NoError(t, err)

	clock := clockwork.NewFakeClock()
	advanceClock := func(d time.Duration) {
		// Wait for the run loop to actually wait on the clock ticker before advancing. If we advance before
		// the loop waits on the ticker, it may never tick.
		clock.BlockUntil(1)
		clock.Advance(d)
	}

	stsClient := &fakeSTSClient{
		clock: clock,
	}

	// Create a configurator with a fake clock and STS client.
	c, err := NewConfigurator(ctx, svc, integrationSvc, nil /*alertService*/, WithClock(clock), WithSTSClient(stsClient))
	require.NoError(t, err)
	require.True(t, c.IsUsed())

	// Set the GenerateOIDCTokenFn to a dumb faked function.
	c.SetGenerateOIDCTokenFn(func(ctx context.Context, integration string) (string, error) {
		return uuid.NewString(), nil
	})

	provider := c.CredentialsProvider()

	checkRetrieveCredentials := func(t require.TestingT, expectErr error) {
		_, err := provider.Retrieve(ctx)
		assert.ErrorIs(t, err, expectErr)
	}
	checkRetrieveCredentialsWithExpiry := func(t require.TestingT, expectExpiry time.Time) {
		creds, err := provider.Retrieve(ctx)
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

	// Assert that credentials can be retrieved when everything is happy.
	// EventuallyWithT is necessary to allow credentialsCache.run to be
	// scheduled after SetGenerateOIDCTokenFn above.
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

	// Test a scenario where STS is returning errors in two different 10-minute windows: the first surrounding
	// the expected cert refresh time, and the second surrounding the cert expiry time.
	// In this case the credentials cache should refresh the certs somewhere between those two outages, and
	// clients should never see an error retrieving credentials.
	expectedRefreshTime := newCredentialExpiry.Add(-refreshBeforeExpirationPeriod)
	credentialsUpdated := false
	done := newCredentialExpiry.Add(10 * time.Minute)
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
					creds, err := provider.Retrieve(ctx)
					assert.NoError(t, err)
					assert.WithinDuration(t, expectedExpiry, creds.Expires, 2*time.Minute)
				}, waitFor, tick)
				credentialsUpdated = true
			}
		}

		// Assert that there is never an error getting credentials.
		checkRetrieveCredentials(t, nil)

	}
}

// TestDraftConfigurator models the way the connection tester will use the
// configurator to synchronously get credentials for the current draft
// ExternalAuditStorageSpec.
func TestDraftConfigurator(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			Cloud: true,
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.ExternalAuditStorage: {Enabled: true},
			},
		},
	})

	mem, err := memory.New(memory.Config{})
	require.NoError(t, err)

	// Pre-req: existing AWS OIDC integration
	integrationSvc, err := local.NewIntegrationsService(mem)
	require.NoError(t, err)
	oidcIntegration := testOIDCIntegration(t)
	_, err = integrationSvc.CreateIntegration(ctx, oidcIntegration)
	require.NoError(t, err)

	// Pre-req: existing draft ExternalAuditStorage configuration
	draftConfig := testDraftExternalAuditStorage(t)
	svc := local.NewExternalAuditStorageService(mem)
	_, err = svc.UpsertDraftExternalAuditStorage(ctx, draftConfig)
	require.NoError(t, err)

	clock := clockwork.NewFakeClock()
	stsClient := &fakeSTSClient{
		clock: clock,
	}

	// Create a draft configurator with a fake clock and STS client.
	c, err := NewDraftConfigurator(ctx, svc, integrationSvc, WithClock(clock), WithSTSClient(stsClient))
	require.NoError(t, err)
	require.True(t, c.IsUsed())

	// Set the GenerateOIDCTokenFn to a faked function for the test.
	c.SetGenerateOIDCTokenFn(func(ctx context.Context, integration string) (string, error) {
		// Can sleep here to confirm that WaitForFirstCredentials works.
		// time.Sleep(time.Second)
		return uuid.NewString(), nil
	})

	// Wait for the first set of credentials to be ready.
	c.WaitForFirstCredentials(ctx)

	// Get credentials, make sure there's no error and the expiry looks right.
	provider := c.CredentialsProvider()
	creds, err := provider.Retrieve(ctx)
	require.NoError(t, err)
	require.WithinDuration(t, clock.Now().Add(TokenLifetime), creds.Expires, time.Minute)
}

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
