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
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/secretsmanager/secretsmanageriface"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// MockSecretsManagerClientConfig is the config for MockSecretsManagerClient.
type MockSecretsManagerClientConfig struct {
	Region  string
	Account string
	Clock   clockwork.Clock
}

// SetDefaults sets defaults.
func (c *MockSecretsManagerClientConfig) SetDefaults() {
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

// MockSecretsManagerClient is a mock implementation of
// secretsmanageriface.SecretsManagerAPI that makes AWSSecretsManager a
// functional in-memory Secrets.
//
// Only used for testing.
type MockSecretsManagerClient struct {
	secretsmanageriface.SecretsManagerAPI

	cfg     MockSecretsManagerClientConfig
	secrets map[string]*secretsmanager.DescribeSecretOutput
	values  map[string]*secretsmanager.GetSecretValueOutput
	mu      sync.RWMutex
}

// NewMockSecretsManagerClient creates a new MockSecretsManagerClient.
func NewMockSecretsManagerClient(cfg MockSecretsManagerClientConfig) *MockSecretsManagerClient {
	cfg.SetDefaults()
	return &MockSecretsManagerClient{
		cfg:     cfg,
		secrets: make(map[string]*secretsmanager.DescribeSecretOutput),
		values:  make(map[string]*secretsmanager.GetSecretValueOutput),
	}
}

func (m *MockSecretsManagerClient) CreateSecretWithContext(_ context.Context, input *secretsmanager.CreateSecretInput, _ ...request.Option) (*secretsmanager.CreateSecretOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := aws.StringValue(input.Name)
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
		defaultKMS, _ := defaultKMSKeyID(aws.StringValue(m.secrets[key].ARN))
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

func (m *MockSecretsManagerClient) DescribeSecretWithContext(_ context.Context, input *secretsmanager.DescribeSecretInput, _ ...request.Option) (*secretsmanager.DescribeSecretOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := aws.StringValue(input.SecretId)
	output, found := m.secrets[key]
	if !found {
		return nil, trace.NotFound("%v not found", key)
	}
	return output, nil
}

func (m *MockSecretsManagerClient) UpdateSecretWithContext(_ context.Context, input *secretsmanager.UpdateSecretInput, _ ...request.Option) (*secretsmanager.UpdateSecretOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := aws.StringValue(input.SecretId)
	if _, found := m.secrets[key]; !found {
		return nil, trace.NotFound("%v not found", key)
	}

	m.secrets[key].KmsKeyId = input.KmsKeyId
	return &secretsmanager.UpdateSecretOutput{
		ARN:  m.secrets[key].ARN,
		Name: m.secrets[key].Name,
	}, nil
}

func (m *MockSecretsManagerClient) DeleteSecretWithContext(_ context.Context, input *secretsmanager.DeleteSecretInput, _ ...request.Option) (*secretsmanager.DeleteSecretOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := aws.StringValue(input.SecretId)
	output, found := m.secrets[key]
	if !found {
		return nil, trace.NotFound("%v not found", key)
	}

	delete(m.secrets, key)
	for versionID, value := range m.values {
		if aws.StringValue(value.Name) == key {
			delete(m.values, versionID)
		}
	}
	return &secretsmanager.DeleteSecretOutput{
		ARN:  output.ARN,
		Name: output.Name,
	}, nil
}

func (m *MockSecretsManagerClient) GetSecretValueWithContext(_ context.Context, input *secretsmanager.GetSecretValueInput, _ ...request.Option) (*secretsmanager.GetSecretValueOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := aws.StringValue(input.SecretId)
	if _, found := m.secrets[key]; !found {
		return nil, trace.NotFound("secret %v not found", key)
	}

	versionID := aws.StringValue(input.VersionId)

	// Find version ID by version stage.
	if aws.StringValue(input.VersionStage) != "" {
		for matchVersionID, stages := range m.secrets[key].VersionIdsToStages {
			if len(stages) > 0 && aws.StringValue(stages[0]) == aws.StringValue(input.VersionStage) {
				versionID = matchVersionID
				break
			}
		}
	}

	output, found := m.values[versionID]
	if !found {
		return nil, trace.NotFound("version %v not found", aws.StringValue(input.VersionId))
	}
	return output, nil
}

func (m *MockSecretsManagerClient) PutSecretValueWithContext(_ context.Context, input *secretsmanager.PutSecretValueInput, _ ...request.Option) (*secretsmanager.PutSecretValueOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := aws.StringValue(input.SecretId)
	if _, found := m.secrets[key]; !found {
		return nil, trace.NotFound("%v not found", key)
	}

	// Test client token before putting new value.
	if aws.StringValue(input.ClientRequestToken) != "" {
		if _, found := m.values[aws.StringValue(input.ClientRequestToken)]; found {
			return nil, trace.BadParameter("version %v is already created", aws.StringValue(input.ClientRequestToken))
		}
	}

	newVersionID := m.putValue(key, input.SecretBinary, input.ClientRequestToken, m.cfg.Clock.Now())
	return &secretsmanager.PutSecretValueOutput{
		ARN:           m.secrets[key].ARN,
		Name:          m.secrets[key].Name,
		VersionId:     newVersionID,
		VersionStages: aws.StringSlice([]string{stageCurrent}),
	}, nil
}

// putValue is a helper function to add a new value for the secret with
// provided key and returns the version ID of the new value.
func (m *MockSecretsManagerClient) putValue(key string, binary []byte, versionID *string, createdAt time.Time) *string {
	if aws.StringValue(versionID) == "" {
		versionID = aws.String(uuid.NewString())
	}

	m.values[aws.StringValue(versionID)] = &secretsmanager.GetSecretValueOutput{
		ARN:          m.secrets[key].ARN,
		Name:         m.secrets[key].Name,
		CreatedDate:  aws.Time(createdAt),
		SecretBinary: binary,
		VersionId:    versionID,
	}

	newVersionsMap := map[string][]*string{
		aws.StringValue(versionID): aws.StringSlice([]string{stageCurrent}),
	}
	for oldVersion, stages := range m.secrets[key].VersionIdsToStages {
		if len(stages) > 0 && aws.StringValue(stages[0]) == stageCurrent { // Demote current to previous.
			newVersionsMap[oldVersion] = aws.StringSlice([]string{stagePrevious})
		}
	}
	m.secrets[key].VersionIdsToStages = newVersionsMap
	return versionID
}
