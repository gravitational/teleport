// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsoidc

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/integrations/externalauditstorage/easconfig"
)

// ConfigureExternalAuditStorageClient is an interface for the AWS client methods
// used by ConfigureExternalAuditStorage.
type ConfigureExternalAuditStorageClient interface {
	PutRolePolicy(context.Context, *iam.PutRolePolicyInput, ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error)
	GetCallerIdentity(context.Context, *sts.GetCallerIdentityInput, ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

// DefaultConfigureExternalAuditStorageClient wraps an iam and sts client to
// implement ConfigureExternalAuditStorageClient.
type DefaultConfigureExternalAuditStorageClient struct {
	Iam *iam.Client
	Sts *sts.Client
}

// PutRolePolicy adds or updates an inline policy document that is embedded in
// the specified IAM role.
func (d *DefaultConfigureExternalAuditStorageClient) PutRolePolicy(ctx context.Context, input *iam.PutRolePolicyInput, opts ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error) {
	return d.Iam.PutRolePolicy(ctx, input, opts...)
}

// GetCallerIdentity returns details about the IAM user or role whose
// credentials are used to call the operation.
func (d *DefaultConfigureExternalAuditStorageClient) GetCallerIdentity(ctx context.Context, input *sts.GetCallerIdentityInput, opts ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return d.Sts.GetCallerIdentity(ctx, input, opts...)
}

// ConfigureExternalAuditStorage attaches an IAM policy with necessary permissions
// for the ExternalAuditStorage feature to an existing IAM role associated with an
// AWS OIDC integration.
func ConfigureExternalAuditStorage(
	ctx context.Context,
	clt ConfigureExternalAuditStorageClient,
	params *easconfig.ExternalAuditStorageConfiguration,
) error {
	fmt.Println("\nConfiguring necessary IAM permissions for External Audit Storage")

	policyCfg := &awslib.ExternalAuditStoragePolicyConfig{
		Partition:           params.Partition,
		Region:              params.Region,
		AthenaWorkgroupName: params.AthenaWorkgroup,
		GlueDatabaseName:    params.GlueDatabase,
		GlueTableName:       params.GlueTable,
	}

	var err error
	bucketARN, wildcardARN, err := s3URIToResourceARNs(params.Partition, params.AuditEventsURI)
	if err != nil {
		return trace.Wrap(err, "parsing audit events URI")
	}
	policyCfg.S3ARNs = append(policyCfg.S3ARNs, bucketARN, wildcardARN)
	bucketARN, wildcardARN, err = s3URIToResourceARNs(params.Partition, params.SessionRecordingsURI)
	if err != nil {
		return trace.Wrap(err, "parsing session recordings URI")
	}
	policyCfg.S3ARNs = append(policyCfg.S3ARNs, bucketARN, wildcardARN)
	bucketARN, wildcardARN, err = s3URIToResourceARNs(params.Partition, params.AthenaResultsURI)
	if err != nil {
		return trace.Wrap(err, "parsing athena results URI")
	}
	policyCfg.S3ARNs = append(policyCfg.S3ARNs, bucketARN, wildcardARN)
	policyCfg.S3ARNs = utils.Deduplicate(policyCfg.S3ARNs)

	stsResp, err := clt.GetCallerIdentity(ctx, nil)
	if err != nil {
		return trace.Wrap(err, "attempting to find caller's AWS account ID: call to sts:GetCallerIdentity failed")
	}
	policyCfg.Account = aws.ToString(stsResp.Account)

	policyDoc, err := awslib.PolicyDocumentForExternalAuditStorage(policyCfg)
	if err != nil {
		return trace.Wrap(err)
	}
	policyDocString, err := policyDoc.Marshal()
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("Attaching inline policy %s to role %s\n", params.Policy, params.Role)
	_, err = clt.PutRolePolicy(ctx, &iam.PutRolePolicyInput{
		PolicyName:     &params.Policy,
		RoleName:       &params.Role,
		PolicyDocument: &policyDocString,
	})
	if err != nil {
		err = awslib.ConvertIAMv2Error(err)
		if trace.IsNotFound(err) {
			return trace.NotFound("role %q not found", params.Role)
		}
		return trace.Wrap(err)
	}

	return nil
}

// s3URIToResourceARNs takes a URI for an s3 bucket with an optional path
// prefix, and returns two AWS s3 resource ARNS. The first is the ARN of the
// bucket, the second is a wildcard ARN matching all objects within the bucket
// and prefix.
// E.g. s3://bucketname/folder -> arn:aws:s3:::bucketname, arn:aws:s3:::bucketname/folder/*
func s3URIToResourceARNs(partition, uri string) (string, string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", "", trace.BadParameter("parsing S3 URI: %v", err)
	}

	if u.Scheme != "s3" {
		return "", "", trace.BadParameter("URI scheme must be s3")
	}

	bucket := u.Host
	bucketARN := arn.ARN{
		Partition: partition,
		Service:   "s3",
		Resource:  bucket,
	}

	resourcePath := bucket
	if folder := strings.Trim(u.Path, "/"); len(folder) > 0 {
		resourcePath += "/" + folder
	}
	resourcePath += "/*"
	wildcardARN := arn.ARN{
		Partition: partition,
		Service:   "s3",
		Resource:  resourcePath,
	}

	return bucketARN.String(), wildcardARN.String(), nil
}
