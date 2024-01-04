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

package cloud

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientGetAWSSessionIntegration(t *testing.T) {
	dummyIntegration := "integration-test"
	dummyRegion := "test-region-123"

	t.Run("without an integration session provider, must return a missing aws integration session provider error", func(t *testing.T) {
		ctx := context.Background()

		clients, err := NewClients()
		require.NoError(t, err)

		t.Cleanup(func() { require.NoError(t, clients.Close()) })

		_, err = clients.GetAWSSession(ctx, "us-region-2", WithCredentialsMaybeIntegration("integration-test"))
		require.True(t, trace.IsBadParameter(err), "expected err to be BadParameter, got %+v", err)
		require.ErrorContains(t, err, "missing aws integration session provider")
	})

	t.Run("with an integration session provider, must return the session", func(t *testing.T) {
		ctx := context.Background()
		dummySession := &awssession.Session{
			Config: &aws.Config{
				Region: &dummyRegion,
			},
		}

		clients, err := NewClients(WithAWSIntegrationSessionProvider(func(ctx context.Context, region, integration string) (*awssession.Session, error) {
			assert.Equal(t, dummyIntegration, integration)
			assert.Equal(t, dummyRegion, region)
			return dummySession, nil
		}))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, clients.Close()) })

		sess, err := clients.GetAWSSession(ctx, dummyRegion, WithCredentialsMaybeIntegration("integration-test"))
		require.NoError(t, err)
		require.Equal(t, dummySession, sess)
	})

	t.Run("with an integration session provider, but using an empty integration falls back to ambient credentials, must not call the integration session provider", func(t *testing.T) {
		ctx := context.Background()

		clients, err := NewClients(WithAWSIntegrationSessionProvider(func(ctx context.Context, region, integration string) (*awssession.Session, error) {
			assert.Fail(t, "should not be called")
			return nil, nil
		}))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, clients.Close()) })

		sess, err := clients.GetAWSSession(ctx, dummyRegion, WithCredentialsMaybeIntegration(""))
		require.NoError(t, err)
		require.NotNil(t, sess)
	})

	t.Run("with an integration session provider, but using ambient credentials, must not call the integration session provider", func(t *testing.T) {
		ctx := context.Background()

		clients, err := NewClients(WithAWSIntegrationSessionProvider(func(ctx context.Context, region, integration string) (*awssession.Session, error) {
			assert.Fail(t, "should not be called")
			return nil, nil
		}))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, clients.Close()) })

		sess, err := clients.GetAWSSession(ctx, dummyRegion, WithAmbientCredentials())
		require.NoError(t, err)
		require.NotNil(t, sess)
	})

	t.Run("with an integration session provider, but no credential source defined", func(t *testing.T) {
		ctx := context.Background()

		clients, err := NewClients(WithAWSIntegrationSessionProvider(func(ctx context.Context, region, integration string) (*awssession.Session, error) {
			assert.Fail(t, "should not be called")
			return nil, nil
		}))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, clients.Close()) })

		_, err = clients.GetAWSSession(ctx, dummyRegion)
		require.Error(t, err)
		require.ErrorContains(t, err, "missing credentials source")
	})
}
