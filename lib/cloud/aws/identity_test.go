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
	"errors"
	"testing"

	stsv2 "github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/gravitational/trace"
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
			inARN:        "arn:aws:iam::123456789012:role/custom/path/EC2ReadOnly",
			outIdentity:  Role{},
			outName:      "EC2ReadOnly",
			outAccountID: "123456789012",
			outPartition: "aws",
			outType:      "role",
		},
		{
			description:  "assumed role identity",
			inARN:        "arn:aws:sts::123456789012:assumed-role/DatabaseAccess/i-1234567890",
			outIdentity:  Role{},
			outName:      "DatabaseAccess",
			outAccountID: "123456789012",
			outPartition: "aws",
			outType:      "assumed-role",
		},
		{
			description:  "user identity",
			inARN:        "arn:aws-us-gov:iam::123456789012:user/custom/path/alice",
			outIdentity:  User{},
			outName:      "alice",
			outAccountID: "123456789012",
			outPartition: "aws-us-gov",
			outType:      "user",
		},
		{
			description:  "unsupported identity",
			inARN:        "arn:aws:iam::123456789012:group/readers",
			outIdentity:  Unknown{},
			outName:      "readers",
			outAccountID: "123456789012",
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

// TestGetIdentityWithClientV2 verifies parsing of AWS identity received from STS API.
func TestGetIdentityWithClientV2(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		description  string
		inARN        string
		inErr        error
		errCheck     require.ErrorAssertionFunc
		outIdentity  Identity
		outName      string
		outAccountID string
		outPartition string
		outType      string
	}{
		{
			description:  "role identity",
			errCheck:     require.NoError,
			inARN:        "arn:aws:iam::123456789012:role/custom/path/EC2ReadOnly",
			outIdentity:  Role{},
			outName:      "EC2ReadOnly",
			outAccountID: "123456789012",
			outPartition: "aws",
			outType:      "role",
		},
		{
			description:  "assumed role identity",
			errCheck:     require.NoError,
			inARN:        "arn:aws:sts::123456789012:assumed-role/DatabaseAccess/i-1234567890",
			outIdentity:  Role{},
			outName:      "DatabaseAccess",
			outAccountID: "123456789012",
			outPartition: "aws",
			outType:      "assumed-role",
		},
		{
			description:  "user identity",
			errCheck:     require.NoError,
			inARN:        "arn:aws-us-gov:iam::123456789012:user/custom/path/alice",
			outIdentity:  User{},
			outName:      "alice",
			outAccountID: "123456789012",
			outPartition: "aws-us-gov",
			outType:      "user",
		},
		{
			description:  "unsupported identity",
			errCheck:     require.NoError,
			inARN:        "arn:aws:iam::123456789012:group/readers",
			outIdentity:  Unknown{},
			outName:      "readers",
			outAccountID: "123456789012",
			outPartition: "aws",
			outType:      "group",
		},
		{
			description: "instance without identity",
			inErr:       errors.New("no EC2 IMDS role found"),
			errCheck: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(tt, trace.IsNotFound(err), "expected a not found error, got=%v", err)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			stsClient := mockSTSV2Client{
				arn: test.inARN,
				err: test.inErr,
			}
			identity, err := GetIdentityWithClientV2(ctx, stsClient)
			test.errCheck(t, err)
			if err != nil {
				return
			}

			require.IsType(t, test.outIdentity, identity)
			require.Equal(t, test.outName, identity.GetName())
			require.Equal(t, test.outAccountID, identity.GetAccountID())
			require.Equal(t, test.outPartition, identity.GetPartition())
			require.Equal(t, test.outType, identity.GetType())
		})
	}
}

type mockSTSV2Client struct {
	arn string
	err error
}

func (m mockSTSV2Client) GetCallerIdentity(ctx context.Context, params *stsv2.GetCallerIdentityInput, optFns ...func(*stsv2.Options)) (*stsv2.GetCallerIdentityOutput, error) {
	if m.err != nil {
		return nil, m.err
	}

	return &stsv2.GetCallerIdentityOutput{
		Arn: aws.String(m.arn),
	}, nil
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
