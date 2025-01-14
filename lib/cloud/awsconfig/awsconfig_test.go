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
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

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

func (m *mockAssumeRoleAPIClient) AssumeRoleWithWebIdentity(ctx context.Context, in *sts.AssumeRoleWithWebIdentityInput, _ ...func(*sts.Options)) (*sts.AssumeRoleWithWebIdentityOutput, error) {
	expiry := time.Now().Add(60 * time.Minute)
	return &sts.AssumeRoleWithWebIdentityOutput{
		Credentials: &ststypes.Credentials{
			AccessKeyId:     in.RoleArn,
			SecretAccessKey: in.WebIdentityToken,
			SessionToken:    aws.String("token"),
			Expiration:      &expiry,
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

	awsOIDCIntegration, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: "integration-test"},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN: "arn:aws:sts::123456789012:role/TestRole",
		},
	)
	require.NoError(t, err)
	fakeIntegrationClt := fakeOIDCIntegrationClient{
		getIntegrationFn: func(context.Context, string) (types.Integration, error) {
			return awsOIDCIntegration, nil
		},
		getTokenFn: func(context.Context, string) (string, error) {
			return "oidc-token", nil
		},
	}

	stsClt := func(cfg aws.Config) STSClient {
		return &mockAssumeRoleAPIClient{}
	}

	t.Run("without an integration client, must return missing credential provider error", func(t *testing.T) {
		ctx := context.Background()
		_, err := provider.GetConfig(ctx, dummyRegion, WithCredentialsMaybeIntegration(dummyIntegration))
		require.True(t, trace.IsBadParameter(err), "unexpected error: %v", err)
		require.ErrorContains(t, err, "missing AWS OIDC integration client")
	})

	t.Run("with an integration client, must return integration fetch error", func(t *testing.T) {
		ctx := context.Background()

		fakeIntegrationClt := fakeIntegrationClt
		fakeIntegrationClt.getIntegrationFn = func(context.Context, string) (types.Integration, error) {
			return nil, trace.NotFound("integration not found")
		}
		_, err := provider.GetConfig(ctx, dummyRegion,
			WithCredentialsMaybeIntegration(dummyIntegration),
			WithOIDCIntegrationClient(&fakeIntegrationClt),
			WithSTSClientProvider(stsClt),
		)
		require.Error(t, err)
		require.ErrorContains(t, err, "integration not found")
	})

	t.Run("with an integration client, must check for AWS integration subkind", func(t *testing.T) {
		ctx := context.Background()

		azureIntegration, err := types.NewIntegrationAzureOIDC(
			types.Metadata{Name: "integration-test"},
			&types.AzureOIDCIntegrationSpecV1{
				TenantID: "abc",
				ClientID: "123",
			},
		)
		require.NoError(t, err)
		fakeIntegrationClt := fakeIntegrationClt
		fakeIntegrationClt.getIntegrationFn = func(context.Context, string) (types.Integration, error) {
			return azureIntegration, nil
		}
		_, err = provider.GetConfig(ctx, dummyRegion,
			WithCredentialsMaybeIntegration(dummyIntegration),
			WithOIDCIntegrationClient(&fakeIntegrationClt),
			WithSTSClientProvider(stsClt),
		)
		require.Error(t, err)
		require.ErrorContains(t, err, "invalid integration subkind")
	})

	t.Run("with an integration client, must return token generation errors", func(t *testing.T) {
		ctx := context.Background()
		fakeIntegrationClt := fakeIntegrationClt
		fakeIntegrationClt.getTokenFn = func(context.Context, string) (string, error) {
			return "", trace.BadParameter("failed to generate OIDC token")
		}
		_, err = provider.GetConfig(ctx, dummyRegion,
			WithCredentialsMaybeIntegration(dummyIntegration),
			WithOIDCIntegrationClient(&fakeIntegrationClt),
			WithSTSClientProvider(stsClt),
		)
		require.Error(t, err)
		require.ErrorContains(t, err, "failed to generate OIDC token")
	})

	t.Run("with an integration client, must return the credentials", func(t *testing.T) {
		ctx := context.Background()

		cfg, err := provider.GetConfig(ctx, dummyRegion,
			WithCredentialsMaybeIntegration(dummyIntegration),
			WithOIDCIntegrationClient(&fakeIntegrationClt),
			WithSTSClientProvider(stsClt),
		)
		require.NoError(t, err)
		creds, err := cfg.Credentials.Retrieve(ctx)
		require.NoError(t, err)
		require.Equal(t, "oidc-token", creds.SecretAccessKey)
	})

	t.Run("with an integration credential provider assuming a role, must return assumed role credentials", func(t *testing.T) {
		ctx := context.Background()

		cfg, err := provider.GetConfig(ctx, dummyRegion,
			WithCredentialsMaybeIntegration(dummyIntegration),
			WithOIDCIntegrationClient(&fakeIntegrationClt),
			WithAssumeRole("roleA", "abc123"),
			WithSTSClientProvider(stsClt),
		)
		require.NoError(t, err)
		creds, err := cfg.Credentials.Retrieve(ctx)
		require.NoError(t, err)
		require.Equal(t, "role: roleA, externalID: abc123", creds.AccessKeyID)
		require.Equal(t, "fake-session-token", creds.SessionToken)
	})

	t.Run("with an integration credential provider assuming a role, must limit role chain length", func(t *testing.T) {
		ctx := context.Background()
		_, err := provider.GetConfig(ctx, dummyRegion,
			WithCredentialsMaybeIntegration(dummyIntegration),
			WithOIDCIntegrationClient(&fakeIntegrationClt),
			WithAssumeRole("roleA", "abc123"),
			WithAssumeRole("roleB", "abc123"),
			WithAssumeRole("roleC", "abc123"),
			WithSTSClientProvider(stsClt),
		)
		require.Error(t, err)
		require.ErrorContains(t, err, "role chain contains more than 2 roles")
	})

	t.Run("with an integration credential provider, but using an empty integration falls back to ambient credentials", func(t *testing.T) {
		ctx := context.Background()

		_, err := provider.GetConfig(ctx, dummyRegion,
			WithCredentialsMaybeIntegration(""),
			WithOIDCIntegrationClient(&fakeOIDCIntegrationClient{unauth: true}),
		)
		require.NoError(t, err)
	})

	t.Run("with an integration credential provider, but using ambient credentials", func(t *testing.T) {
		ctx := context.Background()

		_, err := provider.GetConfig(ctx, dummyRegion,
			WithAmbientCredentials(),
			WithOIDCIntegrationClient(&fakeOIDCIntegrationClient{unauth: true}),
		)
		require.NoError(t, err)
	})

	t.Run("with an integration credential provider, but no credential source", func(t *testing.T) {
		ctx := context.Background()

		_, err := provider.GetConfig(ctx, dummyRegion,
			WithOIDCIntegrationClient(&fakeOIDCIntegrationClient{unauth: true}),
		)
		require.Error(t, err)
		require.ErrorContains(t, err, "missing credentials source")
	})
}

func TestNewCacheKey(t *testing.T) {
	roleChain := []AssumeRole{
		{RoleARN: "roleA"},
		{RoleARN: "roleB", ExternalID: "abc123"},
	}
	got, err := newCacheKey("integration-name", roleChain...)
	require.NoError(t, err)
	want := strings.TrimSpace(`
{"integration":"integration-name","role_chain":[{"role_arn":"roleA","external_id":""},{"role_arn":"roleB","external_id":"abc123"}]}
`)
	require.Equal(t, want, got)
}

type fakeOIDCIntegrationClient struct {
	unauth bool

	getIntegrationFn func(context.Context, string) (types.Integration, error)
	getTokenFn       func(context.Context, string) (string, error)
}

func (f *fakeOIDCIntegrationClient) GetIntegration(ctx context.Context, name string) (types.Integration, error) {
	if f.unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	return f.getIntegrationFn(ctx, name)
}

func (f *fakeOIDCIntegrationClient) GenerateAWSOIDCToken(ctx context.Context, integrationName string) (string, error) {
	if f.unauth {
		return "", trace.AccessDenied("unauthorized")
	}
	return f.getTokenFn(ctx, integrationName)
}
