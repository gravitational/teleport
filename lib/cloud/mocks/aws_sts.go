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
	"slices"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/cloud/awsconfig"
)

// STSClient mocks the AWS STS API for AWS SDK v1 and v2.
// Callers can use it in tests for both the v1 and v2 interfaces.
// This is useful when some services still use SDK v1 while others use v2 SDK,
// so that all assumed roles can be recorded in one place.
// For example:
//
// clt := &STSClient{}
// a.stsClientV1 = &clt.STSClientV1
// b.stsClientV2 = clt
// ...
// gotRoles := clt.GetAssumedRoleARNs() // returns roles that were assumed with either v1 or v2 client.
type STSClient struct {
	STSClientV1

	// credentialProvider is only set when a chain of assumed roles is used.
	credentialProvider aws.CredentialsProvider
	// recordFn records the role and external ID when a role is assumed.
	// It is only set when a chain of assumed roles is used, so that all assumed
	// roles can be centralized and observed in tests.
	recordFn func(roleARN, externalID string)
}

func (m *STSClient) AssumeRole(ctx context.Context, in *sts.AssumeRoleInput, optFns ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
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
			AccessKeyId:     in.RoleArn,
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
	m.STSClientV1.mu.Lock()
	defer m.STSClientV1.mu.Unlock()
	if !slices.Contains(m.assumedRoleARNs, roleARN) {
		m.assumedRoleARNs = append(m.assumedRoleARNs, roleARN)
		m.assumedRoleExternalIDs = append(m.assumedRoleExternalIDs, externalID)
	}
}

func newAssumeRoleClientProviderFunc(base *STSClient) awsconfig.AssumeRoleClientProviderFunc {
	return func(cfg aws.Config) stscreds.AssumeRoleAPIClient {
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
