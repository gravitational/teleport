// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package awsoidc

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ssoadmin"
	"github.com/gravitational/trace"
)

type IDCClient interface {
	ListInstances(ctx context.Context, params *ssoadmin.ListInstancesInput, optFns ...func(*ssoadmin.Options)) (*ssoadmin.ListInstancesOutput, error)
	DescribeInstance(ctx context.Context, params *ssoadmin.DescribeInstanceInput, optFns ...func(*ssoadmin.Options)) (*ssoadmin.DescribeInstanceOutput, error)
}

func NewIDCClient(ctx context.Context, req *AWSClientRequest) (IDCClient, error) {
	cfg, err := newAWSConfig(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ssoadmin.NewFromConfig(*cfg), nil
}
