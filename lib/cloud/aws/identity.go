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
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"

	"github.com/gravitational/trace"
)

// Identity represents an AWS IAM identity such as user or role.
type Identity interface {
	// GetName returns the identity name.
	GetName() string
	// GetAccountID returns the AWS account ID the identity belongs to.
	GetAccountID() string
	// GetPartition returns the AWS partition the identity resides in.
	GetPartition() string
	// GetType returns the identity resource type.
	GetType() string
	// Stringer provides textual representation of identity.
	fmt.Stringer
}

// User represents an AWS IAM user identity.
type User struct {
	identityBase
}

// Role represents an AWS IAM role identity.
type Role struct {
	identityBase
}

// Unknown represents an unknown/unsupported AWS IAM identity.
type Unknown struct {
	identityBase
}

type identityBase struct {
	arn arn.ARN
}

// GetName returns the identity name.
func (i identityBase) GetName() string {
	parts := strings.Split(i.arn.Resource, "/")
	// EC2 instances running on AWS with attached IAM role will have
	// assumed-role identity with ARN like:
	// arn:aws:sts::1234567890:assumed-role/DatabaseAccess/i-1234567890
	if parts[0] == "assumed-role" && len(parts) > 2 {
		return parts[1]
	}
	// Resource can include a path and the name is its last component e.g.
	// arn:aws:iam::1234567890:role/path/to/customrole
	return parts[len(parts)-1]
}

// GetAccountID returns the identity account ID.
func (i identityBase) GetAccountID() string {
	return i.arn.AccountID
}

// GetPartition returns the identity AWS partition.
func (i identityBase) GetPartition() string {
	return i.arn.Partition
}

// GetType returns the identity resource type.
func (i identityBase) GetType() string {
	return strings.Split(i.arn.Resource, "/")[0]
}

// String returns the AWS identity ARN.
func (i identityBase) String() string {
	return i.arn.String()
}

// GetIdentityWithClient determines AWS identity of this Teleport process
// using the provided STS API client.
func GetIdentityWithClient(ctx context.Context, stsClient stsiface.STSAPI) (Identity, error) {
	out, err := stsClient.GetCallerIdentityWithContext(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return IdentityFromArn(aws.StringValue(out.Arn))
}

// IdentityFromArn returns an `Identity` interface based on the provided ARN.
func IdentityFromArn(arnString string) (Identity, error) {
	parsedARN, err := arn.Parse(arnString)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	parts := strings.Split(parsedARN.Resource, "/")
	switch parts[0] {
	case "role", "assumed-role":
		return Role{
			identityBase: identityBase{
				arn: parsedARN,
			},
		}, nil
	case "user":
		return User{
			identityBase: identityBase{
				arn: parsedARN,
			},
		}, nil
	}

	return Unknown{
		identityBase: identityBase{
			arn: parsedARN,
		},
	}, nil
}
