/*
Copyright 2022 Gravitational, Inc.

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

package secrets

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestAWSSecretsManager(t *testing.T) {
	createFunc := func(_ context.Context) (Secrets, error) {
		secrets, err := NewAWSSecretsManager(AWSSecretsManagerConfig{
			Client: NewMockSecretsManagerClient(MockSecretsManagerClientConfig{}),
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return secrets, nil
	}

	// Run common test suite for Secrets.
	secretsTestSuite(t, createFunc)

	t.Run("bad key", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		secrets, err := createFunc(ctx)
		require.NoError(t, err)

		veryLongKey := strings.Repeat("abcdef", 100)

		require.True(t, trace.IsBadParameter(secrets.CreateOrUpdate(ctx, veryLongKey, "value")))
		require.True(t, trace.IsBadParameter(secrets.Delete(ctx, veryLongKey)))
		require.True(t, trace.IsBadParameter(secrets.PutValue(ctx, veryLongKey, "value", "")))
		_, err = secrets.GetValue(ctx, veryLongKey, CurrentVersion)
		require.True(t, trace.IsBadParameter(err))
	})

	t.Run("KMS change", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		// Create secret for the first time with default KMS.
		client := NewMockSecretsManagerClient(MockSecretsManagerClientConfig{})
		secrets, err := NewAWSSecretsManager(AWSSecretsManagerConfig{
			Client: client,
		})
		require.NoError(t, err)
		require.NoError(t, secrets.CreateOrUpdate(ctx, "key", "value"))

		output1, err := client.DescribeSecretWithContext(ctx, &secretsmanager.DescribeSecretInput{
			SecretId: aws.String("teleport/key"),
		})
		require.NoError(t, err)
		require.Equal(t, "arn:aws:kms:us-east-1:123456789012:alias/aws/secretsmanager", aws.StringValue(output1.KmsKeyId))

		// Create secret for the second time with custom KMS. Create returns
		// IsAlreadyExists but KMSKeyID should be updated.
		secrets, err = NewAWSSecretsManager(AWSSecretsManagerConfig{
			Client:   client,
			KMSKeyID: "customKMS",
		})
		require.NoError(t, err)
		require.True(t, trace.IsAlreadyExists(secrets.CreateOrUpdate(ctx, "key", "value")))

		output2, err := client.DescribeSecretWithContext(ctx, &secretsmanager.DescribeSecretInput{
			SecretId: aws.String("teleport/key"),
		})
		require.NoError(t, err)
		require.Equal(t, "customKMS", aws.StringValue(output2.KmsKeyId))

		// Verify the versions are kept.
		require.Equal(t, output1.VersionIdsToStages, output2.VersionIdsToStages)
	})
}
