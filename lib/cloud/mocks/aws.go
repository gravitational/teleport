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
	"net/http"
	"net/url"
	"slices"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
)

// STSClientV1 mocks AWS STS API for AWS SDK v1.
type STSClientV1 struct {
	stsiface.STSAPI
	ARN                    string
	URL                    *url.URL
	assumedRoleARNs        []string
	assumedRoleExternalIDs []string
	mu                     sync.Mutex
}

func (m *STSClientV1) GetAssumedRoleARNs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.assumedRoleARNs
}

func (m *STSClientV1) GetAssumedRoleExternalIDs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.assumedRoleExternalIDs
}

func (m *STSClientV1) ResetAssumeRoleHistory() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.assumedRoleARNs = nil
	m.assumedRoleExternalIDs = nil
}

func (m *STSClientV1) GetCallerIdentityWithContext(aws.Context, *sts.GetCallerIdentityInput, ...request.Option) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{
		Arn: aws.String(m.ARN),
	}, nil
}

func (m *STSClientV1) AssumeRole(in *sts.AssumeRoleInput) (*sts.AssumeRoleOutput, error) {
	return m.AssumeRoleWithContext(context.Background(), in)
}

func (m *STSClientV1) AssumeRoleWithContext(ctx aws.Context, in *sts.AssumeRoleInput, _ ...request.Option) (*sts.AssumeRoleOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !slices.Contains(m.assumedRoleARNs, aws.StringValue(in.RoleArn)) {
		m.assumedRoleARNs = append(m.assumedRoleARNs, aws.StringValue(in.RoleArn))
		m.assumedRoleExternalIDs = append(m.assumedRoleExternalIDs, aws.StringValue(in.ExternalId))
	}
	expiry := time.Now().Add(60 * time.Minute)
	return &sts.AssumeRoleOutput{
		Credentials: &sts.Credentials{
			AccessKeyId:     aws.String("FAKEACCESSKEYID"),
			SecretAccessKey: aws.String("secret"),
			SessionToken:    aws.String("token"),
			Expiration:      &expiry,
		},
	}, nil
}

func (m *STSClientV1) GetCallerIdentityRequest(req *sts.GetCallerIdentityInput) (*request.Request, *sts.GetCallerIdentityOutput) {
	return &request.Request{
		HTTPRequest: &http.Request{
			Header: http.Header{},
			URL:    m.URL,
		},
		Operation: &request.Operation{
			Name:       "GetCallerIdentity",
			HTTPMethod: "POST",
			HTTPPath:   "/",
		},
		Handlers: request.Handlers{},
	}, nil
}
