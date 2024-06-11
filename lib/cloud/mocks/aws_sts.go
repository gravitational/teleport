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

package mocks

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"

	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
)

type STSAPIMock struct {
	libcloudaws.STSAPI

	CallerIdentityARN string

	assumedRoleARNs        []string
	assumedRoleExternalIDs []string
	mu                     sync.Mutex
}

func (m *STSAPIMock) GetAssumedRoleARNs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.assumedRoleARNs
}

func (m *STSAPIMock) GetAssumedRoleExternalIDs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.assumedRoleExternalIDs
}

func (m *STSAPIMock) ResetAssumeRoleHistory() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.assumedRoleARNs = nil
	m.assumedRoleExternalIDs = nil
}

func (m *STSAPIMock) GetCallerIdentity(context.Context, *sts.GetCallerIdentityInput, ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{
		Arn: aws.String(m.CallerIdentityARN),
	}, nil
}

func (m *STSAPIMock) AssumeRole(_ context.Context, in *sts.AssumeRoleInput, _ ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !slices.Contains(m.assumedRoleARNs, aws.ToString(in.RoleArn)) {
		m.assumedRoleARNs = append(m.assumedRoleARNs, aws.ToString(in.RoleArn))
		m.assumedRoleExternalIDs = append(m.assumedRoleExternalIDs, aws.ToString(in.ExternalId))
	}
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
