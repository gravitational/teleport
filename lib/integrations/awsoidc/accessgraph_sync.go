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

package awsoidc

import (
	"context"
	"io"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/gravitational/trace"

	awsutils "github.com/gravitational/teleport/api/utils/aws"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/provisioning"
	"github.com/gravitational/teleport/lib/cloud/provisioning/awsactions"
	"github.com/gravitational/teleport/lib/utils/aws/iamutils"
	"github.com/gravitational/teleport/lib/utils/aws/stsutils"
)

const (
	// defaultPolicyNameForTAGSync is the default name for the Inline TAG Policy added to the IntegrationRole.
	defaultPolicyNameForTAGSync = "AccessGraphSyncAccess"
)

// AccessGraphAWSIAMConfigureRequest is a request to configure the required Policies to use the TAG AWS Sync.
type AccessGraphAWSIAMConfigureRequest struct {
	// IntegrationRole is the Integration's AWS Role used to set up Teleport as an OIDC IdP.
	IntegrationRole string

	// IntegrationRoleTAGPolicy is the Policy Name that is created to allow access to call AWS APIs.
	// Defaults to "AccessGraphSyncAccess"
	IntegrationRoleTAGPolicy string

	// AccountID is the AWS Account ID.
	AccountID string

	// AutoConfirm skips user confirmation of the operation plan if true.
	AutoConfirm bool

	// SQSQueueURL is the URL of the SQS queue to use for the Identity Security Activity Center.
	SQSQueueURL string
	// CloudTrailBucketARN is the name of the S3 bucket to use for the Identity Security Activity Center.
	CloudTrailBucketARN string
	// KMSKeyARNs is the ARN of the KMS key to use for decrypting the Identity Security Activity Center data.
	KMSKeyARNs []string

	// stdout is used to override stdout output in tests.
	stdout io.Writer
}

var (
	sqsQueueRegex = regexp.MustCompile(`^https://sqs\.([a-z0-9-]+)\.amazonaws\.com\/[0-9]{12}\/([a-zA-Z0-9_-]+)$`)
)

// IsValidSQSURL checks if the provided URL is a valid SQS queue URL.
// The URL must be in the format: https://sqs.<region>.amazonaws.com/<account-id>/<queue-name>
func IsValidSQSURL(url string) bool {
	return sqsQueueRegex.MatchString(url)
}

// CheckAndSetDefaults ensures the required fields are present.
func (r *AccessGraphAWSIAMConfigureRequest) CheckAndSetDefaults() error {
	if r.IntegrationRole == "" {
		return trace.BadParameter("integration role is required")
	}

	if r.IntegrationRoleTAGPolicy == "" {
		r.IntegrationRoleTAGPolicy = defaultPolicyNameForTAGSync
	}

	if r.SQSQueueURL != "" || r.CloudTrailBucketARN != "" {
		if r.SQSQueueURL == "" || r.CloudTrailBucketARN == "" {
			return trace.BadParameter("Both CloudTrailBucketARN and SQSQueueURL must be set")
		}
		if !IsValidSQSURL(r.SQSQueueURL) {
			return trace.BadParameter("SQSQueueURL must be a valid SQS URL in the format https://sqs.<region>.amazonaws.com/<account-id>/<queue-name>")
		}

		if a, err := arn.Parse(r.CloudTrailBucketARN); err != nil || a.Service != "s3" || a.Resource == "" {
			return trace.BadParameter("CloudTrailBucketARN must be a valid S3 bucket ARN")
		}

		for _, kmsKeyARN := range r.KMSKeyARNs {
			if a, err := arn.Parse(kmsKeyARN); err != nil || a.Service != "kms" || a.Resource == "" {
				return trace.BadParameter("%q must be valid KMS key ARN", kmsKeyARN)
			}
		}
	}

	return nil
}

// AccessGraphIAMConfigureClient describes the required methods to create the IAM Policies
// required for enrolling Access Graph AWS Sync into Teleport.
type AccessGraphIAMConfigureClient interface {
	CallerIdentityGetter
	// PutRolePolicy creates or replaces a Policy by its name in a IAM Role.
	PutRolePolicy(ctx context.Context, params *iam.PutRolePolicyInput, optFns ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error)
}

type defaultTAGIAMConfigureClient struct {
	CallerIdentityGetter
	*iam.Client
}

// NewAccessGraphIAMConfigureClient creates a new TAGIAMConfigureClient.
func NewAccessGraphIAMConfigureClient(ctx context.Context) (AccessGraphIAMConfigureClient, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("" /* region */))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &defaultTAGIAMConfigureClient{
		CallerIdentityGetter: stsutils.NewFromConfig(cfg),
		Client:               iamutils.NewFromConfig(cfg),
	}, nil
}

// ConfigureAccessGraphSyncIAM sets up the roles required for Teleport to be able to pool
// AWS resources into Teleport.
// The following actions must be allowed by the IAM Role assigned in the Client.
//   - iam:PutRolePolicy
func ConfigureAccessGraphSyncIAM(ctx context.Context, clt AccessGraphIAMConfigureClient, req AccessGraphAWSIAMConfigureRequest) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if err := CheckAccountID(ctx, clt, req.AccountID); err != nil {
		return trace.Wrap(err)
	}

	statements := []*awslib.Statement{awslib.StatementAccessGraphAWSSync()}

	if req.SQSQueueURL != "" {
		// Extract the region and queue name from the SQS URL.
		matches := sqsQueueRegex.FindStringSubmatch(req.SQSQueueURL)
		if len(matches) != 3 {
			return trace.BadParameter("invalid SQSQueueURL format")
		}
		region := matches[1]
		queueName := matches[2]

		if awsutils.IsValidRegion(region) != nil {
			return trace.BadParameter("SQSQueueURL must contain a valid AWS region")
		}

		arnString := arn.ARN{
			Partition: awsutils.GetPartitionFromRegion(region),
			Service:   "sqs",
			Region:    region,
			AccountID: req.AccountID,
			Resource:  queueName,
		}.String()

		statements = append(statements, awslib.StatementAccessGraphAWSSyncSQS(arnString))
	}
	if req.CloudTrailBucketARN != "" {
		statements = append(statements, awslib.StatementAccessGraphAWSSyncS3BucketDownload(req.CloudTrailBucketARN))
	}

	if len(req.KMSKeyARNs) > 0 {
		statements = append(statements, awslib.StatementKMSDecrypt(req.KMSKeyARNs))
	}

	policy := awslib.NewPolicyDocument(
		statements...,
	)
	putRolePolicy, err := awsactions.PutRolePolicy(clt, req.IntegrationRoleTAGPolicy, req.IntegrationRole, policy)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(provisioning.Run(ctx, provisioning.OperationConfig{
		Name: "access-graph-aws-iam",
		Actions: []provisioning.Action{
			*putRolePolicy,
		},
		AutoConfirm: req.AutoConfirm,
		Output:      req.stdout,
	}))
}
