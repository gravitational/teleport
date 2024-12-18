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
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

type mockCredentialProvider struct {
	cred aws.Credentials
}

func (m *mockCredentialProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	return m.cred, nil
}

func TestGetConfigIntegration(t *testing.T) {
	t.Parallel()
	dummyIntegration := "integration-test"
	dummyRegion := "test-region-123"

	t.Run("without an integration credential provider, must return missing credential provider error", func(t *testing.T) {
		ctx := context.Background()
		_, err := GetConfig(ctx, dummyRegion, WithCredentialsMaybeIntegration(dummyIntegration))
		require.True(t, trace.IsBadParameter(err), "unexpected error: %v", err)
		require.ErrorContains(t, err, "missing aws integration credential provider")
	})

	t.Run("with an integration credential provider, must return the credentials", func(t *testing.T) {
		ctx := context.Background()

		cfg, err := GetConfig(ctx, dummyRegion,
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

	t.Run("with an integration credential provider, but using an empty integration falls back to ambient credentials", func(t *testing.T) {
		ctx := context.Background()

		_, err := GetConfig(ctx, dummyRegion,
			WithCredentialsMaybeIntegration(""),
			WithIntegrationCredentialProvider(func(ctx context.Context, region, integration string) (aws.CredentialsProvider, error) {
				require.Fail(t, "this function should not be called")
				return nil, nil
			}))
		require.NoError(t, err)
	})

	t.Run("with an integration credential provider, but using ambient credentials", func(t *testing.T) {
		ctx := context.Background()

		_, err := GetConfig(ctx, dummyRegion,
			WithAmbientCredentials(),
			WithIntegrationCredentialProvider(func(ctx context.Context, region, integration string) (aws.CredentialsProvider, error) {
				require.Fail(t, "this function should not be called")
				return nil, nil
			}))
		require.NoError(t, err)
	})

	t.Run("with an integration credential provider, but no credential source", func(t *testing.T) {
		ctx := context.Background()

		_, err := GetConfig(ctx, dummyRegion,
			WithIntegrationCredentialProvider(func(ctx context.Context, region, integration string) (aws.CredentialsProvider, error) {
				require.Fail(t, "this function should not be called")
				return nil, nil
			}))
		require.Error(t, err)
		require.ErrorContains(t, err, "missing credentials source")
	})
}
