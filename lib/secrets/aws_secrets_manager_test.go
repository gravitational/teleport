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
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestAWSSecretsManager(t *testing.T) {
	// Run common tests from suite.
	secretsTestSuite(t, func(_ context.Context) (Secrets, error) {
		secrets, err := NewAWSSecretsManager(AWSSecretsManagerConfig{
			Client: NewMockSecretsManagerClient(MockSecretsManagerClientConfig{}),
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return secrets, nil
	})

	t.Run("KMS change", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		client := NewMockSecretsManagerClient(MockSecretsManagerClientConfig{})

		// Create for the first time with default KMS.
		secrets, err := NewAWSSecretsManager(AWSSecretsManagerConfig{
			Client: client,
		})
		require.NoError(t, err)
		require.NoError(t, secrets.Create(ctx, "key", "value"))

		output1, err := client.DescribeSecretWithContext(ctx, &secretsmanager.DescribeSecretInput{
			SecretId: aws.String("teleport/key"),
		})
		require.NoError(t, err)
		require.Equal(t, "arn:aws:kms:us-east-1:1234567890:alias/aws/secretsmanager", aws.StringValue(output1.KmsKeyId))

		// Create for the second time with custom KMS. Create returns
		// IsAlreadyExists but KMSKeyID should be updated.
		secrets, err = NewAWSSecretsManager(AWSSecretsManagerConfig{
			Client:   client,
			KMSKeyID: "customKMS",
		})
		require.NoError(t, err)
		require.True(t, trace.IsAlreadyExists(secrets.Create(ctx, "key", "value")))

		output2, err := client.DescribeSecretWithContext(ctx, &secretsmanager.DescribeSecretInput{
			SecretId: aws.String("teleport/key"),
		})
		require.NoError(t, err)
		require.Equal(t, "customKMS", aws.StringValue(output2.KmsKeyId))

		// Verify the versions are kept.
		require.Equal(t, output1.VersionIdsToStages, output2.VersionIdsToStages)
	})
}
