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

package cloud

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"

	"github.com/gravitational/trace"
)

// AWSIdentity represents an AWS IAM identity such as user or role.
type AWSIdentity interface {
	// GetName returns the identity name.
	GetName() string
	// GetAccountID returns the AWS account ID the identity belongs to.
	GetAccountID() string
	// GetPartition returns the AWS partition the identity resides in.
	GetPartition() string
	// Stringer provides textual representation of identity.
	fmt.Stringer
}

// AWSUser represents an AWS IAM user identity.
type AWSUser struct {
	awsBase
}

// AWSRole represents an AWS IAM role identity.
type AWSRole struct {
	awsBase
}

// AWSUnknown represents an unknown/unsupported AWS IAM identity.
type AWSUnknown struct {
	awsBase
}

type awsBase struct {
	arn arn.ARN
}

// GetName returns the identity name.
func (i awsBase) GetName() string {
	// Resource can include a path and the name is its last component e.g.
	// arn:aws:iam::1234567890:role/path/to/customrole
	parts := strings.Split(i.arn.Resource, "/")
	return parts[len(parts)-1]
}

// GetAccountID returns the identity account ID.
func (i awsBase) GetAccountID() string {
	return i.arn.AccountID
}

// GetPartition returns the identity AWS partition.
func (i awsBase) GetPartition() string {
	return i.arn.Partition
}

// String returns the AWS identity ARN.
func (i awsBase) String() string {
	return i.arn.String()
}

// GetAWSIdentity determines the AWS identity of this Teleport process.
func GetAWSIdentity(ctx context.Context) (AWSIdentity, error) {
	sess, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return GetAWSIdentityWithClient(ctx, sts.New(sess))
}

// GetAWSIdentityWithClient determines AWS identity of this Teleport process
// using the provided STS API client.
func GetAWSIdentityWithClient(ctx context.Context, stsClient stsiface.STSAPI) (AWSIdentity, error) {
	out, err := stsClient.GetCallerIdentityWithContext(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	parsedARN, err := arn.Parse(aws.StringValue(out.Arn))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	parts := strings.Split(parsedARN.Resource, "/")
	switch parts[0] {
	case "role":
		return AWSRole{
			awsBase: awsBase{
				arn: parsedARN,
			},
		}, nil
	case "user":
		return AWSUser{
			awsBase: awsBase{
				arn: parsedARN,
			},
		}, nil
	}
	return AWSUnknown{
		awsBase: awsBase{
			arn: parsedARN,
		},
	}, nil
}
