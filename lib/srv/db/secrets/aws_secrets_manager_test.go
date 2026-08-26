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

package secrets

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/cloud/mocks"
)

func TestAWSSecretsManager(t *testing.T) {
	createFunc := func(_ context.Context) (Secrets, error) {
		secrets, err := NewAWSSecretsManager(AWSSecretsManagerConfig{
			Client:      mocks.NewSecretsManagerClient(mocks.SecretsManagerClientConfig{}),
			ClusterName: "example.teleport.sh",
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
		client := mocks.NewSecretsManagerClient(mocks.SecretsManagerClientConfig{})
		secrets, err := NewAWSSecretsManager(AWSSecretsManagerConfig{
			Client:      client,
			ClusterName: "example.teleport.sh",
		})
		require.NoError(t, err)
		require.NoError(t, secrets.CreateOrUpdate(ctx, "key", "value"))

		output1, err := client.DescribeSecret(ctx, &secretsmanager.DescribeSecretInput{
			SecretId: aws.String("teleport/key"),
		})
		require.NoError(t, err)
		require.Equal(t, "arn:aws:kms:us-east-1:123456789012:alias/aws/secretsmanager", aws.ToString(output1.KmsKeyId))

		// Create secret for the second time with custom KMS. Create returns
		// IsAlreadyExists but KMSKeyID should be updated.
		secrets, err = NewAWSSecretsManager(AWSSecretsManagerConfig{
			Client:      client,
			KMSKeyID:    "customKMS",
			ClusterName: "example.teleport.sh",
		})
		require.NoError(t, err)
		require.True(t, trace.IsAlreadyExists(secrets.CreateOrUpdate(ctx, "key", "value")))

		output2, err := client.DescribeSecret(ctx, &secretsmanager.DescribeSecretInput{
			SecretId: aws.String("teleport/key"),
		})
		require.NoError(t, err)
		require.Equal(t, "customKMS", aws.ToString(output2.KmsKeyId))

		// Verify the versions are kept.
		require.Equal(t, output1.VersionIdsToStages, output2.VersionIdsToStages)
	})
}
