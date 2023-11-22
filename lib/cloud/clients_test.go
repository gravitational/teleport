/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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

	t.Run("without an integration, must return a missing aws integration session provider error", func(t *testing.T) {
		ctx := context.Background()

		clients, err := NewClients()
		require.NoError(t, err)

		t.Cleanup(func() { require.NoError(t, clients.Close()) })

		_, err = clients.GetAWSSession(ctx, "us-region-2", WithIntegration("integration-test"))
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

		sess, err := clients.GetAWSSession(ctx, dummyRegion, WithIntegration("integration-test"))
		require.NoError(t, err)
		require.Equal(t, dummySession, sess)
	})

	t.Run("with an integration session provider, but no integration is requested, must not call the integration session provider", func(t *testing.T) {
		ctx := context.Background()

		clients, err := NewClients(WithAWSIntegrationSessionProvider(func(ctx context.Context, region, integration string) (*awssession.Session, error) {
			assert.Fail(t, "should not be called")
			return nil, nil
		}))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, clients.Close()) })

		sess, err := clients.GetAWSSession(ctx, dummyRegion)
		require.NoError(t, err)
		require.NotNil(t, sess)
	})
}
