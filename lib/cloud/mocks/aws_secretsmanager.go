/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package mocks

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// SecretsManagerClientConfig is the config for [SecretsManagerClient].
type SecretsManagerClientConfig struct {
	Region  string
	Account string
	Clock   clockwork.Clock
}

// setDefaults sets defaults.
func (c *SecretsManagerClientConfig) setDefaults() {
	if c.Region == "" {
		c.Region = "us-east-1"
	}
	if c.Account == "" {
		c.Account = "123456789012"
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewFakeClock()
	}
}

// SecretsManagerClient is a mock implementation of
// secretsmanageriface.SecretsManagerAPI that makes AWSSecretsManager a
// functional in-memory Secrets.
//
// Only used for testing.
type SecretsManagerClient struct {
	cfg     SecretsManagerClientConfig
	secrets map[string]*secretsmanager.DescribeSecretOutput
	values  map[string]*secretsmanager.GetSecretValueOutput
	mu      sync.RWMutex
}

// NewSecretsManagerClient creates a new [SecretsManagerClient].
func NewSecretsManagerClient(cfg SecretsManagerClientConfig) *SecretsManagerClient {
	cfg.setDefaults()
	return &SecretsManagerClient{
		cfg:     cfg,
		secrets: make(map[string]*secretsmanager.DescribeSecretOutput),
		values:  make(map[string]*secretsmanager.GetSecretValueOutput),
	}
}

func (m *SecretsManagerClient) CreateSecret(_ context.Context, input *secretsmanager.CreateSecretInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := aws.ToString(input.Name)
	if _, found := m.secrets[key]; found {
		return nil, trace.AlreadyExists("%v already exists", key)
	}

	// Create secret.
	now := m.cfg.Clock.Now()
	m.secrets[key] = &secretsmanager.DescribeSecretOutput{
		ARN:         aws.String(fmt.Sprintf("arn:aws:elasticache:%s:%s:user:%s", m.cfg.Region, m.cfg.Account, key)),
		CreatedDate: aws.Time(now),
		KmsKeyId:    input.KmsKeyId,
		Name:        input.Name,
		Tags:        input.Tags,
	}
	if m.secrets[key].KmsKeyId == nil {
		defaultKMS, _ := defaultKMSKeyID(aws.ToString(m.secrets[key].ARN))
		m.secrets[key].KmsKeyId = aws.String(defaultKMS)
	}

	// Create version value.
	var newVersionID *string
	if input.SecretBinary != nil {
		newVersionID = m.putValue(key, input.SecretBinary, input.ClientRequestToken, now)
	}
	return &secretsmanager.CreateSecretOutput{
		ARN:       m.secrets[key].ARN,
		Name:      m.secrets[key].Name,
		VersionId: newVersionID,
	}, nil
}

func (m *SecretsManagerClient) DescribeSecret(_ context.Context, input *secretsmanager.DescribeSecretInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := aws.ToString(input.SecretId)
	output, found := m.secrets[key]
	if !found {
		return nil, trace.NotFound("%v not found", key)
	}
	return output, nil
}

func (m *SecretsManagerClient) UpdateSecret(_ context.Context, input *secretsmanager.UpdateSecretInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.UpdateSecretOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := aws.ToString(input.SecretId)
	if _, found := m.secrets[key]; !found {
		return nil, trace.NotFound("%v not found", key)
	}

	m.secrets[key].KmsKeyId = input.KmsKeyId
	return &secretsmanager.UpdateSecretOutput{
		ARN:  m.secrets[key].ARN,
		Name: m.secrets[key].Name,
	}, nil
}

func (m *SecretsManagerClient) DeleteSecret(_ context.Context, input *secretsmanager.DeleteSecretInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.DeleteSecretOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := aws.ToString(input.SecretId)
	output, found := m.secrets[key]
	if !found {
		return nil, trace.NotFound("%v not found", key)
	}

	delete(m.secrets, key)
	for versionID, value := range m.values {
		if aws.ToString(value.Name) == key {
			delete(m.values, versionID)
		}
	}
	return &secretsmanager.DeleteSecretOutput{
		ARN:  output.ARN,
		Name: output.Name,
	}, nil
}

func (m *SecretsManagerClient) GetSecretValue(_ context.Context, input *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := aws.ToString(input.SecretId)
	if _, found := m.secrets[key]; !found {
		return nil, trace.NotFound("secret %v not found", key)
	}

	versionID := aws.ToString(input.VersionId)

	// Find version ID by version stage.
	if aws.ToString(input.VersionStage) != "" {
		for matchVersionID, stages := range m.secrets[key].VersionIdsToStages {
			if len(stages) > 0 && stages[0] == aws.ToString(input.VersionStage) {
				versionID = matchVersionID
				break
			}
		}
	}

	output, found := m.values[versionID]
	if !found {
		return nil, trace.NotFound("version %v not found", aws.ToString(input.VersionId))
	}
	return output, nil
}

func (m *SecretsManagerClient) PutSecretValue(_ context.Context, input *secretsmanager.PutSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := aws.ToString(input.SecretId)
	if _, found := m.secrets[key]; !found {
		return nil, trace.NotFound("%v not found", key)
	}

	// Test client token before putting new value.
	if aws.ToString(input.ClientRequestToken) != "" {
		if _, found := m.values[aws.ToString(input.ClientRequestToken)]; found {
			return nil, trace.BadParameter("version %v is already created", aws.ToString(input.ClientRequestToken))
		}
	}

	newVersionID := m.putValue(key, input.SecretBinary, input.ClientRequestToken, m.cfg.Clock.Now())
	return &secretsmanager.PutSecretValueOutput{
		ARN:           m.secrets[key].ARN,
		Name:          m.secrets[key].Name,
		VersionId:     newVersionID,
		VersionStages: []string{stageCurrent},
	}, nil
}

func (m *SecretsManagerClient) TagResource(context.Context, *secretsmanager.TagResourceInput, ...func(*secretsmanager.Options)) (*secretsmanager.TagResourceOutput, error) {
	return nil, trace.NotImplemented("TagResource not implemented")
}

// putValue is a helper function to add a new value for the secret with
// provided key and returns the version ID of the new value.
func (m *SecretsManagerClient) putValue(key string, binary []byte, versionID *string, createdAt time.Time) *string {
	if aws.ToString(versionID) == "" {
		versionID = aws.String(uuid.NewString())
	}

	m.values[aws.ToString(versionID)] = &secretsmanager.GetSecretValueOutput{
		ARN:          m.secrets[key].ARN,
		Name:         m.secrets[key].Name,
		CreatedDate:  aws.Time(createdAt),
		SecretBinary: binary,
		VersionId:    versionID,
	}

	newVersionsMap := map[string][]string{
		aws.ToString(versionID): {stageCurrent},
	}
	for oldVersion, stages := range m.secrets[key].VersionIdsToStages {
		if len(stages) > 0 && stages[0] == stageCurrent { // Demote current to previous.
			newVersionsMap[oldVersion] = []string{stagePrevious}
		}
	}
	m.secrets[key].VersionIdsToStages = newVersionsMap
	return versionID
}

const (
	// stageCurrent is the stage for current version of the secret.
	stageCurrent = "AWSCURRENT"
	// stagePrevious is the stage for previous version of the secret.
	stagePrevious = "AWSPREVIOUS"
)

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
