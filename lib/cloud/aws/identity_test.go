/*
Copyright 2021 Gravitational, Inc.

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

package aws

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"

	"github.com/stretchr/testify/require"
)

// TestGetIdentity verifies parsing of AWS identity received from STS API.
func TestGetIdentity(t *testing.T) {
	tests := []struct {
		description  string
		inARN        string
		outIdentity  Identity
		outName      string
		outAccountID string
		outPartition string
		outType      string
	}{
		{
			description:  "role identity",
			inARN:        "arn:aws:iam::1234567890:role/custom/path/EC2ReadOnly",
			outIdentity:  Role{},
			outName:      "EC2ReadOnly",
			outAccountID: "1234567890",
			outPartition: "aws",
			outType:      "role",
		},
		{
			description:  "assumed role identity",
			inARN:        "arn:aws:sts::1234567890:assumed-role/DatabaseAccess/i-1234567890",
			outIdentity:  Role{},
			outName:      "DatabaseAccess",
			outAccountID: "1234567890",
			outPartition: "aws",
			outType:      "assumed-role",
		},
		{
			description:  "user identity",
			inARN:        "arn:aws-us-gov:iam::1234567890:user/custom/path/alice",
			outIdentity:  User{},
			outName:      "alice",
			outAccountID: "1234567890",
			outPartition: "aws-us-gov",
			outType:      "user",
		},
		{
			description:  "unsupported identity",
			inARN:        "arn:aws:iam::1234567890:group/readers",
			outIdentity:  Unknown{},
			outName:      "readers",
			outAccountID: "1234567890",
			outPartition: "aws",
			outType:      "group",
		},
	}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			identity, err := GetIdentityWithClient(context.Background(), &stsMock{arn: test.inARN})
			require.NoError(t, err)
			require.IsType(t, test.outIdentity, identity)
			require.Equal(t, test.outName, identity.GetName())
			require.Equal(t, test.outAccountID, identity.GetAccountID())
			require.Equal(t, test.outPartition, identity.GetPartition())
			require.Equal(t, test.outType, identity.GetType())
		})
	}
}

type stsMock struct {
	stsiface.STSAPI
	arn string
}

func (m *stsMock) GetCallerIdentityWithContext(aws.Context, *sts.GetCallerIdentityInput, ...request.Option) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{
		Arn: aws.String(m.arn),
	}, nil
}
