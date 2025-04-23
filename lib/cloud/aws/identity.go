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

package aws

import (
	"context"
	"fmt"
	"strings"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/sts"
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

const (
	// ResourceTypeRole is the resource type for an AWS IAM role.
	ResourceTypeRole = "role"
	// ResourceTypeAssumedRole is the resource type for an AWS IAM assumed role.
	ResourceTypeAssumedRole = "assumed-role"
	// ResourceTypeUser is the resource type for an AWS IAM user.
	ResourceTypeUser = "user"
)

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
	// arn:aws:sts::123456789012:assumed-role/DatabaseAccess/i-1234567890
	if parts[0] == ResourceTypeAssumedRole && len(parts) > 2 {
		return parts[1]
	}
	// Resource can include a path and the name is its last component e.g.
	// arn:aws:iam::123456789012:role/path/to/customrole
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

type callerIdentityGetter interface {
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

// GetIdentityWithClient determines AWS identity of this Teleport process
// using the provided STS API client.
func GetIdentityWithClient(ctx context.Context, clt callerIdentityGetter) (Identity, error) {
	out, err := clt.GetCallerIdentity(ctx, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var callerARN string
	if out != nil {
		callerARN = awsv2.ToString(out.Arn)
	}
	return IdentityFromArn(callerARN)
}

// IdentityFromArn returns an `Identity` interface based on the provided ARN.
func IdentityFromArn(arnString string) (Identity, error) {
	parsedARN, err := arn.Parse(arnString)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	parts := strings.Split(parsedARN.Resource, "/")
	switch parts[0] {
	case ResourceTypeRole, ResourceTypeAssumedRole:
		return Role{
			identityBase: identityBase{
				arn: parsedARN,
			},
		}, nil
	case ResourceTypeUser:
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
