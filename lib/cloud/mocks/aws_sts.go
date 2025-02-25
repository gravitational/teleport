/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"net/url"
	"slices"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/cloud/awsconfig"
)

// STSClient mocks the AWS STS API for AWS SDK v2.
type STSClient struct {
	ARN                    string
	URL                    *url.URL
	assumedRoleARNs        []string
	assumedRoleExternalIDs []string
	mu                     sync.Mutex

	Unauth bool
	// credentialProvider is only set when a chain of assumed roles is used.
	credentialProvider aws.CredentialsProvider
	// recordFn records the role and external ID when a role is assumed.
	// It is only set when a chain of assumed roles is used, so that all assumed
	// roles can be centralized and observed in tests.
	recordFn func(roleARN, externalID string)
}

func (m *STSClient) GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}

	return &sts.GetCallerIdentityOutput{
		Arn: aws.String(m.ARN),
	}, nil
}

func (m *STSClient) AssumeRoleWithWebIdentity(ctx context.Context, in *sts.AssumeRoleWithWebIdentityInput, _ ...func(*sts.Options)) (*sts.AssumeRoleWithWebIdentityOutput, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}

	m.record(aws.ToString(in.RoleArn), "")
	expiry := time.Now().Add(60 * time.Minute)
	return &sts.AssumeRoleWithWebIdentityOutput{
		Credentials: &ststypes.Credentials{
			AccessKeyId:     aws.String("WEBIDENTITYFAKEACCESSKEYID"),
			SecretAccessKey: aws.String("secret"),
			SessionToken:    aws.String("token"),
			Expiration:      &expiry,
		},
	}, nil
}

func (m *STSClient) AssumeRole(ctx context.Context, in *sts.AssumeRoleInput, _ ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}

	// Retrieve credentials if we have a credential provider, so that all
	// assume-role providers in a role chain are triggered to call AssumeRole.
	if m.credentialProvider != nil {
		_, err := m.credentialProvider.Retrieve(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	m.record(aws.ToString(in.RoleArn), aws.ToString(in.ExternalId))

	expiry := time.Now().Add(60 * time.Minute)
	return &sts.AssumeRoleOutput{
		Credentials: &ststypes.Credentials{
			AccessKeyId:     aws.String("FAKEACCESSKEYID"),
			SecretAccessKey: aws.String("secret"),
			SessionToken:    aws.String("token"),
			Expiration:      &expiry,
		},
	}, nil
}

// record is a helper function that records the role ARN and external ID for an
// assumed role.
// It delegates to the configured recordFn, if it has one, so that all assumed
// role recordings are centralized for observation in tests.
func (m *STSClient) record(roleARN, externalID string) {
	if m.recordFn != nil {
		m.recordFn(roleARN, externalID)
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !slices.Contains(m.assumedRoleARNs, roleARN) {
		m.assumedRoleARNs = append(m.assumedRoleARNs, roleARN)
		m.assumedRoleExternalIDs = append(m.assumedRoleExternalIDs, externalID)
	}
}

func newAssumeRoleClientProviderFunc(base *STSClient) awsconfig.STSClientProviderFunc {
	return func(cfg aws.Config) awsconfig.STSClient {
		if cfg.Credentials != nil {
			if _, ok := cfg.Credentials.(*stscreds.AssumeRoleProvider); ok {
				// Create a new fake client linked to the old one.
				// Only do this for AssumeRoleProvider to avoid attempting
				// to load the real credential chain.
				return &STSClient{
					credentialProvider: cfg.Credentials,
					recordFn:           base.record,
				}
			}
		}
		return base
	}
}

func (m *STSClient) GetAssumedRoleARNs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.assumedRoleARNs
}

func (m *STSClient) GetAssumedRoleExternalIDs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.assumedRoleExternalIDs
}

func (m *STSClient) ResetAssumeRoleHistory() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.assumedRoleARNs = nil
	m.assumedRoleExternalIDs = nil
}
