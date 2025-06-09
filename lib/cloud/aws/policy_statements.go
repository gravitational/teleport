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
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

var wildcard = "*"
var allResources = []string{wildcard}

// StatementForECSManageService returns the statement that allows managing the ECS Service deployed
// by DeployService (AWS OIDC Integration).
func StatementForECSManageService() *Statement {
	return &Statement{
		Effect: EffectAllow,
		Actions: []string{
			"ecs:DescribeClusters", "ecs:CreateCluster", "ecs:PutClusterCapacityProviders",
			"ecs:DescribeServices", "ecs:CreateService", "ecs:UpdateService", "ecs:ListServices",
			"ecs:RegisterTaskDefinition", "ecs:DescribeTaskDefinition", "ecs:DeregisterTaskDefinition",

			// Required if the account has Resource Tagging Authorization enabled in Amazon ECS.
			"ecs:TagResource",

			// EC2 DescribeSecurityGroups is required so that the user can list the SG and then pick which ones they want to apply to the ECS Service.
			"ec2:DescribeSecurityGroups",

			// IAM CreateServiceLinkedRole is required to ensure the ECS Service linked role exists.
			"iam:CreateServiceLinkedRole",
		},
		Resources: allResources,
	}
}

// StatementForWritingLogs returns the statement that allows the writing logs to CloudWatch.
// This is used by the DeployService (ECS Service) to write teleport logs.
// https://docs.aws.amazon.com/AmazonECS/latest/developerguide/using_awslogs.html
func StatementForWritingLogs() *Statement {
	return &Statement{
		Effect:    EffectAllow,
		Actions:   []string{"logs:CreateLogStream", "logs:PutLogEvents", "logs:CreateLogGroup"},
		Resources: allResources,
	}
}

// StatementForIAMPassRole returns a statement that allows to iam:PassRole the target role.
// Usage example: when setting up the TaskRole for the ECS Task.
// https://docs.aws.amazon.com/AmazonECS/latest/userguide/task-iam-roles.html#specify-task-iam-roles
func StatementForIAMPassRole(targetRole string) *Statement {
	return &Statement{
		Effect:  EffectAllow,
		Actions: SliceOrString{"iam:PassRole"},
		Resources: SliceOrString{
			targetRole,
		},
	}
}

// StatementForECSTaskRoleTrustRelationships returns the Trust Relationship to allow the ECS Tasks service to.
// It allows the usage of this Role by the ECS Tasks service.
func StatementForECSTaskRoleTrustRelationships() *Statement {
	return &Statement{
		Effect:  EffectAllow,
		Actions: SliceOrString{"sts:AssumeRole"},
		Principals: map[string]SliceOrString{
			"Service": {"ecs-tasks.amazonaws.com"},
		},
	}
}

// StatementForRDSDBConnect returns a statement that allows the `rds-db:connect` for all RDS DBs.
func StatementForRDSDBConnect() *Statement {
	return &Statement{
		Effect:    EffectAllow,
		Actions:   SliceOrString{"rds-db:connect"},
		Resources: allResources,
	}
}

// StatementForRDSMetadata returns a statement that allows describing RDS
// instances and clusters for metadata import, as in monitoring AWS tags and
// whether IAM auth is enabled.
func StatementForRDSMetadata() *Statement {
	return &Statement{
		Effect: EffectAllow,
		Actions: SliceOrString{
			"rds:DescribeDBInstances",
			"rds:DescribeDBClusters",
		},
		Resources: allResources,
	}
}

// StatementForEC2InstanceConnectEndpoint returns the statement that allows the flow for accessing
// an EC2 instance using its private IP, using EC2 Instance Connect Endpoint.
func StatementForEC2InstanceConnectEndpoint() *Statement {
	return &Statement{
		Effect: EffectAllow,
		Actions: []string{
			"ec2:DescribeInstances",
			"ec2:DescribeInstanceConnectEndpoints",
			"ec2:DescribeSecurityGroups",

			// Create ICE requires the following actions:
			// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/permissions-for-ec2-instance-connect-endpoint.html
			"ec2:CreateInstanceConnectEndpoint",
			"ec2:CreateTags",
			"ec2:CreateNetworkInterface",
			"iam:CreateServiceLinkedRole",

			"ec2-instance-connect:SendSSHPublicKey",
			"ec2-instance-connect:OpenTunnel",
		},
		Resources: allResources,
	}
}

// StatementForEC2SSMAutoDiscover returns the required statement to enable EC2 Auto Discover using SSM.
func StatementForEC2SSMAutoDiscover() *Statement {
	return &Statement{
		Effect: EffectAllow,
		Actions: []string{
			"ec2:DescribeInstances",
			"ssm:DescribeInstanceInformation",
			"ssm:GetCommandInvocation",
			"ssm:ListCommandInvocations",
			"ssm:SendCommand",
		},
		Resources: allResources,
	}
}

// StatementForAWSAppAccess returns the statement that allows AWS App Access.
// Only IAM Roles with `teleport.dev/integration: Allowed` Tag can be used.
func StatementForAWSAppAccess() *Statement {
	requiredTag := types.TeleportNamespace + "/integration"
	return &Statement{
		Effect: EffectAllow,
		Actions: []string{
			"sts:AssumeRole",
		},
		Resources: allResources,
		Conditions: map[string]StringOrMap{
			"StringEquals": {
				"iam:ResourceTag/" + requiredTag: SliceOrString{"true"},
			},
		},
	}
}

// StatementForEKSAccess returns the statement that allows enrolling of EKS clusters into Teleport.
func StatementForEKSAccess() *Statement {
	return &Statement{
		Effect: EffectAllow,
		Actions: []string{
			"eks:ListClusters",
			"eks:DescribeCluster",
			"eks:ListAccessEntries",
			"eks:CreateAccessEntry",
			"eks:DeleteAccessEntry",
			"eks:AssociateAccessPolicy",
			"eks:TagResource",
		},
		Resources: allResources,
	}
}

// StatementForAWSOIDCRoleTrustRelationship returns the Trust Relationship to allow the OpenID Connect Provider
// set up during the AWS OIDC Onboarding to assume this Role.
func StatementForAWSOIDCRoleTrustRelationship(accountID, providerURL string, audiences []string) *Statement {
	federatedARN := fmt.Sprintf("arn:aws:iam::%s:oidc-provider/%s", accountID, providerURL)
	federatedAudience := fmt.Sprintf("%s:aud", providerURL)

	return &Statement{
		Effect:  EffectAllow,
		Actions: SliceOrString{"sts:AssumeRoleWithWebIdentity"},
		Principals: map[string]SliceOrString{
			"Federated": []string{federatedARN},
		},
		Conditions: map[string]StringOrMap{
			"StringEquals": {
				federatedAudience: audiences,
			},
		},
	}
}

// StatementForListRDSDatabases returns the statement that allows listing RDS DB Clusters and Instances.
func StatementForListRDSDatabases() *Statement {
	return &Statement{
		Effect: EffectAllow,
		Actions: []string{
			"rds:DescribeDBInstances",
			"rds:DescribeDBClusters",
			"ec2:DescribeSecurityGroups",
			"ec2:DescribeSubnets",
			"ec2:DescribeVpcs",
		},
		Resources: allResources,
	}
}

// StatementForS3BucketPublicRead returns the statement that
// allows public/anonynous access to s3 bucket/prefix objects.
func StatementForS3BucketPublicRead(s3bucketName, objectPrefix string) *Statement {
	return &Statement{
		Effect: EffectAllow,
		Principals: StringOrMap{
			wildcard: SliceOrString{},
		},
		Actions: []string{
			"s3:GetObject",
		},
		Resources: []string{
			fmt.Sprintf("arn:aws:s3:::%s/%s/*", s3bucketName, objectPrefix),
		},
	}
}

// ExternalAuditStoragePolicyConfig holds options for the External Audit Storage
// IAM policy.
type ExternalAuditStoragePolicyConfig struct {
	// Partition is the AWS partition to use.
	Partition string
	// Region is the AWS region to use.
	Region string
	// Account is the AWS account ID to use.
	Account string
	// S3ARNs is a list of all S3 resource ARNs used for audit events, session
	// recordings, and Athena query results. For each location, it should include an ARN for the
	// base bucket and another wildcard ARN for all objects within the bucket
	// and an optional path/prefix.
	S3ARNs []string
	// AthenaWorkgroupName is the name of the Athena workgroup used for queries.
	AthenaWorkgroupName string
	// GlueDatabaseName is the name of the AWS Glue database.
	GlueDatabaseName string
	// GlueTabelName is the name of the AWS Glue table.
	GlueTableName string
}

func (c *ExternalAuditStoragePolicyConfig) CheckAndSetDefaults() error {
	if len(c.Partition) == 0 {
		c.Partition = "aws"
	}
	if len(c.Region) == 0 {
		return trace.BadParameter("region is required")
	}
	if len(c.Account) == 0 {
		return trace.BadParameter("account is required")
	}
	if len(c.S3ARNs) < 2 {
		return trace.BadParameter("at least two distinct S3 ARNs are required")
	}
	if len(c.AthenaWorkgroupName) == 0 {
		return trace.BadParameter("athena workgroup name is required")
	}
	if len(c.GlueDatabaseName) == 0 {
		return trace.BadParameter("glue database name is required")
	}
	if len(c.GlueTableName) == 0 {
		return trace.BadParameter("glue table name is required")
	}
	return nil
}

// PolicyDocumentForExternalAuditStorage returns a PolicyDocument with the
// necessary IAM permissions for the External Audit Storage feature.
func PolicyDocumentForExternalAuditStorage(cfg *ExternalAuditStoragePolicyConfig) (*PolicyDocument, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &PolicyDocument{
		Version: PolicyVersion,
		Statements: []*Statement{
			{
				StatementID: "ReadWriteSessionsAndEvents",
				Effect:      EffectAllow,
				Actions: []string{
					"s3:PutObject",
					"s3:GetObject",
					"s3:GetObjectVersion",
					"s3:ListMultipartUploadParts",
					"s3:AbortMultipartUpload",
					"s3:ListBucket",
					"s3:ListBucketVersions",
					"s3:ListBucketMultipartUploads",
					"s3:GetBucketOwnershipControls",
					"s3:GetBucketPublicAccessBlock",
					"s3:GetBucketObjectLockConfiguration",
					"s3:GetBucketVersioning",
					"s3:GetBucketLocation",
				},
				Resources: cfg.S3ARNs,
			},
			{
				StatementID: "AllowAthenaQuery",
				Effect:      EffectAllow,
				Actions: []string{
					"athena:StartQueryExecution",
					"athena:GetQueryResults",
					"athena:GetQueryExecution",
				},
				Resources: []string{
					arn.ARN{
						Partition: cfg.Partition,
						Service:   "athena",
						Region:    cfg.Region,
						AccountID: cfg.Account,
						Resource:  "workgroup/" + cfg.AthenaWorkgroupName,
					}.String(),
				},
			},
			{
				StatementID: "FullAccessOnGlueTable",
				Effect:      EffectAllow,
				Actions: []string{
					"glue:GetTable",
					"glue:GetTableVersion",
					"glue:GetTableVersions",
					"glue:UpdateTable",
				},
				Resources: []string{
					arn.ARN{
						Partition: cfg.Partition,
						Service:   "glue",
						Region:    cfg.Region,
						AccountID: cfg.Account,
						Resource:  "catalog",
					}.String(),
					arn.ARN{
						Partition: cfg.Partition,
						Service:   "glue",
						Region:    cfg.Region,
						AccountID: cfg.Account,
						Resource:  "database/" + cfg.GlueDatabaseName,
					}.String(),
					arn.ARN{
						Partition: cfg.Partition,
						Service:   "glue",
						Region:    cfg.Region,
						AccountID: cfg.Account,
						Resource:  "table/" + cfg.GlueDatabaseName + "/" + cfg.GlueTableName,
					}.String(),
				},
			},
		},
	}, nil
}

// StatementAccessGraphAWSSync returns the statement that allows configuring the AWS Sync feature.
func StatementAccessGraphAWSSync() *Statement {
	return &Statement{
		Effect: EffectAllow,
		Actions: []string{
			// EC2 IAM
			"ec2:DescribeInstances",
			"ec2:DescribeImages",
			"ec2:DescribeTags",
			"ec2:DescribeSnapshots",
			"ec2:DescribeKeyPairs",
			// EKS IAM
			"eks:ListClusters",
			"eks:DescribeCluster",
			"eks:ListAccessEntries",
			"eks:ListAccessPolicies",
			"eks:ListAssociatedAccessPolicies",
			"eks:DescribeAccessEntry",

			// RDS IAM
			"rds:DescribeDBInstances",
			"rds:DescribeDBClusters",
			"rds:ListTagsForResource",
			"rds:DescribeDBProxies",

			// DynamoDB IAM
			"dynamodb:ListTables",
			"dynamodb:DescribeTable",
			// Redshift IAM
			"redshift:DescribeClusters",
			"redshift:Describe*",
			// S3 IAM
			"s3:ListAllMyBuckets",
			"s3:GetBucketPolicy",
			"s3:ListBucket",
			"s3:GetBucketLocation",
			"s3:GetBucketTagging",
			"s3:GetBucketPolicyStatus",
			"s3:GetBucketAcl",

			// IAM IAM
			"iam:ListUsers",
			"iam:GetUser",
			"iam:ListRoles",
			"iam:ListGroups",
			"iam:ListPolicies",
			"iam:ListGroupsForUser",
			"iam:ListInstanceProfiles",
			"iam:ListUserPolicies",
			"iam:GetUserPolicy",
			"iam:ListAttachedUserPolicies",
			"iam:ListGroupPolicies",
			"iam:GetGroupPolicy",
			"iam:ListAttachedGroupPolicies",
			"iam:GetPolicy",
			"iam:GetPolicyVersion",
			"iam:ListRolePolicies",
			"iam:ListAttachedRolePolicies",
			"iam:GetRolePolicy",
			"iam:ListSAMLProviders",
			"iam:GetSAMLProvider",
			"iam:ListOpenIDConnectProviders",
			"iam:GetOpenIDConnectProvider",
		},
		Resources: allResources,
	}
}

// StatementAccessGraphAWSSyncSQS returns the statement that allows
// receiving, deleting, and sending messages to the specified SQS queue.
// This is used for receiving and processing AWS cloud trail logs notifications
// from SQS.
func StatementAccessGraphAWSSyncSQS(sqsQueueARN string) *Statement {
	return &Statement{
		Effect: EffectAllow,
		Actions: []string{
			"sqs:ReceiveMessage",
			"sqs:DeleteMessage",
		},
		Resources: []string{sqsQueueARN},
	}
}

// StatementAccessGraphAWSSyncS3BucketDownload returns the statement that allows downloading
// objects from the specified S3 bucket. This is used for downloading AWS cloud trail logs.
func StatementAccessGraphAWSSyncS3BucketDownload(s3BucketARN string) *Statement {
	return &Statement{
		Effect: EffectAllow,
		Actions: []string{
			"s3:GetObject",
			"s3:GetObjectVersion",
			"s3:ListBucket",
			"s3:ListBucketVersions",
			"s3:GetBucketLocation",
		},
		Resources: []string{s3BucketARN},
	}
}

// StatementAccessGraphAWSSyncKMSDecrypt returns the statement that allows decrypting
// KMS encrypted data. This is used for decrypting AWS cloud trail logs from S3
// and decrypting SQS messages that are encrypted with KMS.
// It allows the following actions:
// - `kms:Decrypt` to decrypt data.
// - `kms:DescribeKey` to get information about the KMS key.
// - `kms:GenerateDataKey` to generate a data key for encryption.
// - `kms:GenerateDataKeyWithoutPlaintext` to generate a data key without plaintext.
func StatementKMSDecrypt(kmsKeysARNs []string) *Statement {
	return &Statement{
		Effect: EffectAllow,
		Actions: []string{
			"kms:Decrypt",
			"kms:DescribeKey",
			"kms:GenerateDataKey",
			"kms:GenerateDataKeyWithoutPlaintext",
		},
		Resources: kmsKeysARNs,
	}
}

// StatementForAWSIdentityCenterAccess returns AWS IAM policy statement that grants
// permissions required for Teleport identity center client.
// TODO(sshah): make the roles more granular by restricting resources scoped to
// particular AWS identity center region+arn.
func StatementForAWSIdentityCenterAccess() *Statement {
	return &Statement{
		StatementID: "TeleportIdentityCenterClient",
		Effect:      EffectAllow,
		Actions: []string{
			// ListAccounts
			"organizations:ListAccounts",
			"organizations:ListAccountsForParent",

			// ListGroupsAndMembers
			"identitystore:ListUsers",
			"identitystore:ListGroups",
			"identitystore:ListGroupMemberships",

			// ListPermissionSetsAndAssignments
			"sso:DescribeInstance",
			"sso:DescribePermissionSet",
			"sso:ListPermissionSets",
			"sso:ListAccountAssignmentsForPrincipal",
			"sso:ListPermissionSetsProvisionedToAccount",

			// CreateAndDeleteAccountAssignment
			"sso:CreateAccountAssignment",
			"sso:DescribeAccountAssignmentCreationStatus",
			"sso:DeleteAccountAssignment",
			"sso:DescribeAccountAssignmentDeletionStatus",
			"iam:AttachRolePolicy",
			"iam:CreateRole",
			"iam:GetRole",
			"iam:ListAttachedRolePolicies",
			"iam:ListRolePolicies",

			// AllowAccountAssignmentOnOwner
			"iam:GetSAMLProvider",

			// ListProvisionedRoles
			"iam:ListRoles",
		},
		Resources: allResources,
	}
}
