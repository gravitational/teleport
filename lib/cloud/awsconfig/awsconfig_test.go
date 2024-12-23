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

package awsconfig

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

type mockCredentialProvider struct {
	cred aws.Credentials
}

func (m *mockCredentialProvider) Retrieve(_ context.Context) (aws.Credentials, error) {
	return m.cred, nil
}

type mockAssumeRoleAPIClient struct{}

func (m *mockAssumeRoleAPIClient) AssumeRole(_ context.Context, params *sts.AssumeRoleInput, optFns ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
	fakeKeyID := fmt.Sprintf("role: %s, externalID: %s", aws.ToString(params.RoleArn), aws.ToString(params.ExternalId))
	return &sts.AssumeRoleOutput{
		AssumedRoleUser: &ststypes.AssumedRoleUser{
			Arn:           params.RoleArn,
			AssumedRoleId: aws.String("role-id"),
		},
		Credentials: &ststypes.Credentials{
			AccessKeyId:     aws.String(fakeKeyID),
			Expiration:      aws.Time(time.Time{}),
			SecretAccessKey: aws.String("fake-secret-access-key"),
			SessionToken:    aws.String("fake-session-token"),
		},
	}, nil
}

func TestGetConfigIntegration(t *testing.T) {
	t.Parallel()

	cache, err := NewCache()
	require.NoError(t, err)
	tests := []struct {
		desc string
		Provider
	}{
		{
			desc:     "uncached",
			Provider: ProviderFunc(GetConfig),
		},
		{
			desc:     "cached",
			Provider: cache,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			testGetConfigIntegration(t, test.Provider)
		})
	}
}

func testGetConfigIntegration(t *testing.T, provider Provider) {
	dummyIntegration := "integration-test"
	dummyRegion := "test-region-123"

	t.Run("without an integration credential provider, must return missing credential provider error", func(t *testing.T) {
		ctx := context.Background()
		_, err := provider.GetConfig(ctx, dummyRegion, WithCredentialsMaybeIntegration(dummyIntegration))
		require.True(t, trace.IsBadParameter(err), "unexpected error: %v", err)
		require.ErrorContains(t, err, "missing aws integration credential provider")
	})

	t.Run("with an integration credential provider, must return the credentials", func(t *testing.T) {
		ctx := context.Background()

		cfg, err := provider.GetConfig(ctx, dummyRegion,
			WithCredentialsMaybeIntegration(dummyIntegration),
			WithIntegrationCredentialProvider(func(ctx context.Context, region, integration string) (aws.CredentialsProvider, error) {
				if region == dummyRegion && integration == dummyIntegration {
					return &mockCredentialProvider{
						cred: aws.Credentials{
							SessionToken: "foo-bar",
						},
					}, nil
				}
				return nil, trace.NotFound("no creds in region %q with integration %q", region, integration)
			}))
		require.NoError(t, err)
		creds, err := cfg.Credentials.Retrieve(ctx)
		require.NoError(t, err)
		require.Equal(t, "foo-bar", creds.SessionToken)
	})

	t.Run("with an integration credential provider assuming a role, must return assumed role credentials", func(t *testing.T) {
		ctx := context.Background()

		cfg, err := provider.GetConfig(ctx, dummyRegion,
			WithCredentialsMaybeIntegration(dummyIntegration),
			WithIntegrationCredentialProvider(func(ctx context.Context, region, integration string) (aws.CredentialsProvider, error) {
				if region == dummyRegion && integration == dummyIntegration {
					return &mockCredentialProvider{
						cred: aws.Credentials{
							SessionToken: "foo-bar",
						},
					}, nil
				}
				return nil, trace.NotFound("no creds in region %q with integration %q", region, integration)
			}),
			WithAssumeRole("roleA", "abc123"),
			WithAssumeRoleClientProviderFunc(func(cfg aws.Config) stscreds.AssumeRoleAPIClient {
				creds, err := cfg.Credentials.Retrieve(context.Background())
				require.NoError(t, err)
				require.Equal(t, "foo-bar", creds.SessionToken)
				return &mockAssumeRoleAPIClient{}
			}),
		)
		require.NoError(t, err)
		creds, err := cfg.Credentials.Retrieve(ctx)
		require.NoError(t, err)
		require.Equal(t, "role: roleA, externalID: abc123", creds.AccessKeyID)
		require.Equal(t, "fake-session-token", creds.SessionToken)
	})

	t.Run("with an integration credential provider, but using an empty integration falls back to ambient credentials", func(t *testing.T) {
		ctx := context.Background()

		_, err := provider.GetConfig(ctx, dummyRegion,
			WithCredentialsMaybeIntegration(""),
			WithIntegrationCredentialProvider(func(ctx context.Context, region, integration string) (aws.CredentialsProvider, error) {
				require.Fail(t, "this function should not be called")
				return nil, nil
			}))
		require.NoError(t, err)
	})

	t.Run("with an integration credential provider, but using ambient credentials", func(t *testing.T) {
		ctx := context.Background()

		_, err := provider.GetConfig(ctx, dummyRegion,
			WithAmbientCredentials(),
			WithIntegrationCredentialProvider(func(ctx context.Context, region, integration string) (aws.CredentialsProvider, error) {
				require.Fail(t, "this function should not be called")
				return nil, nil
			}))
		require.NoError(t, err)
	})

	t.Run("with an integration credential provider, but no credential source", func(t *testing.T) {
		ctx := context.Background()

		_, err := provider.GetConfig(ctx, dummyRegion,
			WithIntegrationCredentialProvider(func(ctx context.Context, region, integration string) (aws.CredentialsProvider, error) {
				require.Fail(t, "this function should not be called")
				return nil, nil
			}))
		require.Error(t, err)
		require.ErrorContains(t, err, "missing credentials source")
	})
}

func TestGetCacheKeyForRoles(t *testing.T) {
	tests := []struct {
		desc    string
		roles   []assumeRole
		want    string
		wantErr string
	}{
		{
			desc: "valid without external ID",
			roles: []assumeRole{
				{roleARN: "roleA"},
				{roleARN: "roleB"},
			},
			want: "roleA||roleB||",
		},
		{
			desc: "valid with external ID",
			roles: []assumeRole{
				{roleARN: "roleA", externalID: "extA"},
				{roleARN: "roleB", externalID: "extB"},
			},
			want: "roleA|extA|roleB|extB|",
		},
		{
			desc:    "invalid role ARN",
			roles:   []assumeRole{{roleARN: "roleA|extA|roleB"}},
			wantErr: "invalid role ARN",
		},
		{
			desc:    "invalid external ID",
			roles:   []assumeRole{{roleARN: "roleA", externalID: "extA|roleB|extB"}},
			wantErr: "invalid external ID",
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			got, err := getCacheKeyForRoles(test.roles)
			if test.wantErr != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, test.wantErr)
				return
			}
			require.Equal(t, test.want, got)
		})
	}
}
