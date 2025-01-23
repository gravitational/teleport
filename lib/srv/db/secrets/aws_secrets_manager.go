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
	"errors"
	"maps"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/secretsmanager/secretsmanageriface"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	libaws "github.com/gravitational/teleport/lib/cloud/aws"
)

// AWSSecretsManagerConfig is the config for AWSSecretsManager.
type AWSSecretsManagerConfig struct {
	// KeyPrefix is the key path prefix for all keys used by Secrets.
	KeyPrefix string `yaml:"key_prefix,omitempty"`
	// KMSKeyID is the AWS KMS key that Secrets Manager uses to encrypt and
	// decrypt the secret value.
	KMSKeyID string `yaml:"kms_key_id,omitempty"`
	// Client is the AWS API client for Secrets Manager.
	Client secretsmanageriface.SecretsManagerAPI
	// ClusterName is the name of the Teleport cluster (for tagging purpose).
	ClusterName string
}

// CheckAndSetDefaults validates the config and sets defaults.
func (c *AWSSecretsManagerConfig) CheckAndSetDefaults() error {
	if c.KeyPrefix == "" {
		c.KeyPrefix = DefaultKeyPrefix
	}
	if c.Client == nil {
		return trace.BadParameter("missing client")
	}
	if c.ClusterName == "" {
		return trace.BadParameter("missing cluster name")
	}
	return nil
}

// AWSSecretsManager is a Secrets store implementation using AWS Secrets
// Manager.
type AWSSecretsManager struct {
	cfg AWSSecretsManagerConfig
}

// NewAWSSecretsManager creates a new Secrets using AWS Secrets Manager.
func NewAWSSecretsManager(cfg AWSSecretsManagerConfig) (*AWSSecretsManager, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &AWSSecretsManager{
		cfg: cfg,
	}, nil
}

// CreateOrUpdate creates a new secret. Implements Secrets.
func (s *AWSSecretsManager) CreateOrUpdate(ctx context.Context, key string, value string) error {
	secretID, err := s.secretID(key)
	if err != nil {
		return trace.Wrap(err)
	}

	input := &secretsmanager.CreateSecretInput{
		Name:               secretID,
		Description:        aws.String("Created by Teleport."),
		ClientRequestToken: aws.String(uuid.New().String()),
		SecretBinary:       []byte(value),

		// Add tags to make it is easier to search Teleport resources.
		Tags: s.makeTags(),
	}
	if s.cfg.KMSKeyID != "" {
		input.KmsKeyId = aws.String(s.cfg.KMSKeyID)
	}

	if _, err := s.cfg.Client.CreateSecretWithContext(ctx, input); err != nil {
		err = convertSecretsManagerError(err)

		switch {
		case trace.IsAlreadyExists(err):
			// If already exists, try to update its settings.
			if updateErr := s.update(ctx, key); updateErr != nil {
				return trace.Wrap(updateErr)
			}

			// Update succeeds. Return trace.IsAlreadyExists.
			return trace.Wrap(err)

		default:
			return trace.Wrap(err)
		}
	}
	return nil
}

func (s *AWSSecretsManager) makeTags() []*secretsmanager.Tag {
	return []*secretsmanager.Tag{
		{
			Key:   aws.String(libaws.TagKeyTeleportCreated),
			Value: aws.String(libaws.TagValueTrue),
		},
		{
			Key:   aws.String(libaws.TagKeyTeleportCluster),
			Value: aws.String(s.cfg.ClusterName),
		},
	}
}

// Delete deletes the secret for the provided path. Implements Secrets.
func (s *AWSSecretsManager) Delete(ctx context.Context, key string) error {
	secretID, err := s.secretID(key)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = s.cfg.Client.DeleteSecretWithContext(ctx, &secretsmanager.DeleteSecretInput{
		SecretId: secretID,

		// Remove secret immediately. Otherwise, secret will be hidden but
		// effective for 7 days before it's actually removed.
		ForceDeleteWithoutRecovery: aws.Bool(true),
	})
	if err != nil {
		return trace.Wrap(convertSecretsManagerError(err))
	}
	return nil
}

// GetValue returns the secret value for provided version. Implements Secrets.
func (s *AWSSecretsManager) GetValue(ctx context.Context, key string, version string) (*Value, error) {
	secretID, err := s.secretID(key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	input := &secretsmanager.GetSecretValueInput{
		SecretId: secretID,
	}

	switch version {
	case "", CurrentVersion:
		input.VersionStage = aws.String(stageCurrent)
	case PreviousVersion:
		input.VersionStage = aws.String(stagePrevious)
	default:
		input.VersionId = aws.String(version)
	}

	output, err := s.cfg.Client.GetSecretValueWithContext(ctx, input)
	if err != nil {
		return nil, trace.Wrap(convertSecretsManagerError(err))
	}

	return &Value{
		Key:       aws.StringValue(output.Name),
		Value:     string(output.SecretBinary),
		Version:   aws.StringValue(output.VersionId),
		CreatedAt: aws.TimeValue(output.CreatedDate),
	}, nil
}

// PutValue creates a new secret version for the secret. Implements Secrets.
func (s *AWSSecretsManager) PutValue(ctx context.Context, key, value, currentVersion string) error {
	secretID, err := s.secretID(key)
	if err != nil {
		return trace.Wrap(err)
	}

	input := &secretsmanager.PutSecretValueInput{
		SecretId:     secretID,
		SecretBinary: []byte(value),
	}

	// Create a new version ID based on current version and use it as
	// ClientRequestToken. This ensures ONLY ONE caller succeeds if multiple
	// calls to PutValue of the same current version are made to AWS. See go
	// doc on ClientRequestToken for more details.
	if currentVersion != "" {
		input.ClientRequestToken = aws.String(uuid.NewMD5(uuid.Nil, []byte(currentVersion)).String())
	} else {
		input.ClientRequestToken = aws.String(uuid.New().String())
	}

	if _, err := s.cfg.Client.PutSecretValueWithContext(ctx, input); err != nil {
		return trace.Wrap(convertSecretsManagerError(err))
	}
	return nil
}

// update updates secret settings for provided key path.
func (s *AWSSecretsManager) update(ctx context.Context, key string) error {
	secretID, err := s.secretID(key)
	if err != nil {
		return trace.Wrap(err)
	}

	secret, err := s.cfg.Client.DescribeSecretWithContext(ctx, &secretsmanager.DescribeSecretInput{
		SecretId: secretID,
	})
	if err != nil {
		return trace.Wrap(convertSecretsManagerError(err))
	}

	if err := s.maybeUpdateTags(ctx, secret); err != nil {
		return trace.Wrap(err)
	}

	configKMSKeyID := s.cfg.KMSKeyID
	if aws.StringValue(secret.KmsKeyId) == configKMSKeyID {
		return nil
	}

	// If configKMSKeyID is empty, but DescribeSecretOutput.KmsKeyId is not,
	// populate configKMSKeyID to the default AWS managed KMS key for Secrets
	// Manager then compare again.
	if configKMSKeyID == "" {
		if configKMSKeyID, err = defaultKMSKeyID(aws.StringValue(secret.ARN)); err != nil {
			return trace.Wrap(err)
		}

		if aws.StringValue(secret.KmsKeyId) == configKMSKeyID {
			return nil
		}
	}

	_, err = s.cfg.Client.UpdateSecretWithContext(ctx, &secretsmanager.UpdateSecretInput{
		SecretId: secretID,
		KmsKeyId: aws.String(configKMSKeyID),
	})
	return trace.Wrap(convertSecretsManagerError(err))
}

func (s *AWSSecretsManager) maybeUpdateTags(ctx context.Context, secert *secretsmanager.DescribeSecretOutput) error {
	wantTags := s.makeTags()
	if maps.Equal(libaws.TagsToLabels(wantTags), libaws.TagsToLabels(secert.Tags)) {
		return nil
	}
	_, err := s.cfg.Client.TagResource(&secretsmanager.TagResourceInput{
		SecretId: secert.ARN,
		Tags:     wantTags,
	})
	return trace.Wrap(convertSecretsManagerError(err))
}

// secretID returns the secret ID in AWS string format.
func (s *AWSSecretsManager) secretID(key string) (*string, error) {
	// Secret names contain 1-512 characters.
	// https://docs.aws.amazon.com/secretsmanager/latest/userguide/reference_limits.html
	secretID := Key(s.cfg.KeyPrefix, key)
	if len(secretID) < 1 || len(secretID) >= 512 {
		return nil, trace.BadParameter("invalid secret ID %s", secretID)
	}
	return aws.String(secretID), nil
}

// defaultKMSKeyID returns the default KMS Key ID for provided secret.
func defaultKMSKeyID(secretARN string) (string, error) {
	parsed, err := arn.Parse(secretARN)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Secrets manager default KMS alias looks like this.
	// arn:aws:kms:us-east-1:1234567890:alias/aws/secretsmanager
	arn := arn.ARN{
		Partition: parsed.Partition,
		Service:   "kms",
		Region:    parsed.Region,
		AccountID: parsed.AccountID,
		Resource:  "alias/aws/secretsmanager",
	}
	return arn.String(), nil
}

// convertSecretsManagerError converts AWS Secrets Manager errors to trace
// errors.
func convertSecretsManagerError(err error) error {
	if err == nil {
		return nil
	}

	var awsError awserr.Error
	if !errors.As(err, &awsError) {
		return trace.Wrap(err)
	}

	// Match by exception code as many errors are sharing the same status code.
	switch awsError.Code() {
	case secretsmanager.ErrCodeResourceExistsException:
		return trace.AlreadyExists(awsError.Error())
	case secretsmanager.ErrCodeResourceNotFoundException:
		return trace.NotFound(awsError.Error())
	}

	// Match by status code.
	return trace.Wrap(libaws.ConvertRequestFailureError(err))
}

const (
	// stageCurrent is the stage for current version of the secret.
	stageCurrent = "AWSCURRENT"
	// stagePrevious is the stage for previous version of the secret.
	stagePrevious = "AWSPREVIOUS"
)
