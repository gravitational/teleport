// Copyright 2022 Gravitational, Inc
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

package aws

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/configurators"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
)

var sortStringsTrans = cmp.Transformer("SortStrings", func(in []string) []string {
	out := append([]string(nil), in...) // Copy input to avoid mutating it
	sort.Strings(out)
	return out
})

func TestAWSIAMDocuments(t *testing.T) {
	userTarget, err := awslib.IdentityFromArn("arn:aws:iam::123456789012:user/example-user")
	require.NoError(t, err)

	role1 := "arn:aws:iam::123456789012:role/role-1"
	role2 := "arn:aws:iam::123456789012:role/role-2"
	role3 := "arn:aws:iam::123456789012:role/role-3"
	role4 := "arn:aws:iam::123456789012:role/role-4"
	role5 := "arn:aws:iam::123456789012:role/role-5"

	roleTarget, err := awslib.IdentityFromArn("arn:aws:iam::123456789012:role/target-role")
	require.NoError(t, err)

	unknownIdentity, err := awslib.IdentityFromArn("arn:aws:iam::123456789012:ec2/example-ec2")
	require.NoError(t, err)

	tests := map[string]struct {
		returnError        bool
		flags              configurators.BootstrapFlags
		fileConfig         *config.FileConfig
		target             awslib.Identity
		statements         []*awslib.Statement
		boundaryStatements []*awslib.Statement
	}{
		"RDSAutoDiscoveryToUser": {
			target: userTarget,
			fileConfig: &config.FileConfig{
				Databases: config.Databases{
					Service: config.Service{EnabledFlag: "true"},
					AWSMatchers: []config.AWSMatcher{
						{Types: []string{services.AWSMatcherRDS}, Regions: []string{"us-west-2"}},
					},
				},
			},
			statements: []*awslib.Statement{
				{Effect: awslib.EffectAllow, Resources: []string{"*"}, Actions: []string{
					"rds:DescribeDBInstances", "rds:ModifyDBInstance",
					"rds:DescribeDBClusters", "rds:ModifyDBCluster",
				}},
				{Effect: awslib.EffectAllow, Resources: []string{userTarget.String()}, Actions: []string{
					"iam:GetUserPolicy", "iam:PutUserPolicy", "iam:DeleteUserPolicy",
				}},
			},
			boundaryStatements: []*awslib.Statement{
				{Effect: awslib.EffectAllow, Resources: []string{"*"}, Actions: []string{
					"rds:DescribeDBInstances", "rds:ModifyDBInstance",
					"rds:DescribeDBClusters", "rds:ModifyDBCluster",
					"rds-db:connect",
				}},
				{Effect: awslib.EffectAllow, Resources: []string{userTarget.String()}, Actions: []string{
					"iam:GetUserPolicy", "iam:PutUserPolicy", "iam:DeleteUserPolicy",
				}},
			},
		},
		"RDSAutoDiscoveryToRole": {
			target: roleTarget,
			fileConfig: &config.FileConfig{
				Databases: config.Databases{
					Service: config.Service{EnabledFlag: "true"},
					AWSMatchers: []config.AWSMatcher{
						{Types: []string{services.AWSMatcherRDS}, Regions: []string{"us-west-2"}},
					},
				},
			},
			statements: []*awslib.Statement{
				{Effect: awslib.EffectAllow, Resources: []string{"*"}, Actions: []string{
					"rds:DescribeDBInstances", "rds:ModifyDBInstance",
					"rds:DescribeDBClusters", "rds:ModifyDBCluster",
				}},
				{Effect: awslib.EffectAllow, Resources: []string{roleTarget.String()}, Actions: []string{
					"iam:GetRolePolicy", "iam:PutRolePolicy", "iam:DeleteRolePolicy",
				}},
			},
			boundaryStatements: []*awslib.Statement{
				{Effect: awslib.EffectAllow, Resources: []string{"*"}, Actions: []string{
					"rds:DescribeDBInstances", "rds:ModifyDBInstance",
					"rds:DescribeDBClusters", "rds:ModifyDBCluster",
					"rds-db:connect",
				}},
				{Effect: awslib.EffectAllow, Resources: []string{roleTarget.String()}, Actions: []string{
					"iam:GetRolePolicy", "iam:PutRolePolicy", "iam:DeleteRolePolicy",
				}},
			},
		},
		"RDS static database": {
			target: roleTarget,
			fileConfig: &config.FileConfig{
				Databases: config.Databases{
					Service: config.Service{EnabledFlag: "true"},
					Databases: []*config.Database{
						{
							Name:     "aurora-1",
							Protocol: "postgres",
							URI:      "aurora-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
						},
					},
				},
			},
			statements: []*awslib.Statement{
				{Effect: awslib.EffectAllow, Resources: []string{"*"}, Actions: []string{
					"rds:DescribeDBInstances", "rds:ModifyDBInstance",
					"rds:DescribeDBClusters", "rds:ModifyDBCluster",
				}},
				{Effect: awslib.EffectAllow, Resources: []string{roleTarget.String()}, Actions: []string{
					"iam:GetRolePolicy", "iam:PutRolePolicy", "iam:DeleteRolePolicy",
				}},
			},
			boundaryStatements: []*awslib.Statement{
				{Effect: awslib.EffectAllow, Resources: []string{"*"}, Actions: []string{
					"rds:DescribeDBInstances", "rds:ModifyDBInstance",
					"rds:DescribeDBClusters", "rds:ModifyDBCluster",
					"rds-db:connect",
				}},
				{Effect: awslib.EffectAllow, Resources: []string{roleTarget.String()}, Actions: []string{
					"iam:GetRolePolicy", "iam:PutRolePolicy", "iam:DeleteRolePolicy",
				}},
			},
		},
		"RedshiftAutoDiscoveryToUser": {
			target: userTarget,
			fileConfig: &config.FileConfig{
				Databases: config.Databases{
					Service: config.Service{EnabledFlag: "true"},
					AWSMatchers: []config.AWSMatcher{
						{Types: []string{services.AWSMatcherRedshift}, Regions: []string{"us-west-2"}},
					},
				},
			},
			statements: []*awslib.Statement{
				{Effect: awslib.EffectAllow, Resources: []string{"*"}, Actions: []string{
					"redshift:DescribeClusters",
				}},
				{Effect: awslib.EffectAllow, Resources: []string{userTarget.String()}, Actions: []string{
					"iam:GetUserPolicy", "iam:PutUserPolicy", "iam:DeleteUserPolicy",
				}},
			},
			boundaryStatements: []*awslib.Statement{
				{Effect: awslib.EffectAllow, Resources: []string{"*"}, Actions: []string{
					"redshift:DescribeClusters", "redshift:GetClusterCredentials",
				}},
				{Effect: awslib.EffectAllow, Resources: []string{userTarget.String()}, Actions: []string{
					"iam:GetUserPolicy", "iam:PutUserPolicy", "iam:DeleteUserPolicy",
				}},
			},
		},
		"RedshiftAutoDiscoveryToRole": {
			target: roleTarget,
			fileConfig: &config.FileConfig{
				Databases: config.Databases{
					Service: config.Service{EnabledFlag: "true"},
					AWSMatchers: []config.AWSMatcher{
						{Types: []string{services.AWSMatcherRedshift}, Regions: []string{"us-west-2"}},
					},
				},
			},
			statements: []*awslib.Statement{
				{Effect: awslib.EffectAllow, Resources: []string{"*"}, Actions: []string{
					"redshift:DescribeClusters",
				}},
				{Effect: awslib.EffectAllow, Resources: []string{roleTarget.String()}, Actions: []string{
					"iam:GetRolePolicy", "iam:PutRolePolicy", "iam:DeleteRolePolicy",
				}},
			},
			boundaryStatements: []*awslib.Statement{
				{Effect: awslib.EffectAllow, Resources: []string{"*"}, Actions: []string{
					"redshift:DescribeClusters", "redshift:GetClusterCredentials",
				}},
				{Effect: awslib.EffectAllow, Resources: []string{roleTarget.String()}, Actions: []string{
					"iam:GetRolePolicy", "iam:PutRolePolicy", "iam:DeleteRolePolicy",
				}},
			},
		},
		"RedshiftDatabases": {
			target: roleTarget,
			fileConfig: &config.FileConfig{
				Databases: config.Databases{
					Service: config.Service{EnabledFlag: "true"},
					Databases: []*config.Database{
						{
							Name:     "redshift-cluster-1",
							Protocol: "postgres",
							URI:      "redshift-cluster-1.abcdefghijkl.us-west-2.redshift.amazonaws.com:5439",
						},
					},
				},
			},
			statements: []*awslib.Statement{
				{Effect: awslib.EffectAllow, Resources: []string{"*"}, Actions: []string{
					"redshift:DescribeClusters",
				}},
				{Effect: awslib.EffectAllow, Resources: []string{roleTarget.String()}, Actions: []string{
					"iam:GetRolePolicy", "iam:PutRolePolicy", "iam:DeleteRolePolicy",
				}},
			},
			boundaryStatements: []*awslib.Statement{
				{Effect: awslib.EffectAllow, Resources: []string{"*"}, Actions: []string{
					"redshift:DescribeClusters", "redshift:GetClusterCredentials",
				}},
				{Effect: awslib.EffectAllow, Resources: []string{roleTarget.String()}, Actions: []string{
					"iam:GetRolePolicy", "iam:PutRolePolicy", "iam:DeleteRolePolicy",
				}},
			},
		},
		"ElastiCache auto discovery": {
			target: roleTarget,
			fileConfig: &config.FileConfig{
				Databases: config.Databases{
					Service: config.Service{EnabledFlag: "true"},
					AWSMatchers: []config.AWSMatcher{
						{Types: []string{services.AWSMatcherElastiCache}, Regions: []string{"us-west-2"}},
					},
				},
			},
			statements: []*awslib.Statement{
				{Effect: awslib.EffectAllow, Resources: []string{"*"}, Actions: []string{
					"elasticache:ListTagsForResource",
					"elasticache:DescribeReplicationGroups",
					"elasticache:DescribeCacheClusters",
					"elasticache:DescribeCacheSubnetGroups",
					"elasticache:DescribeUsers",
					"elasticache:ModifyUser",
				}},
				{
					Effect: awslib.EffectAllow,
					Actions: []string{
						"secretsmanager:DescribeSecret", "secretsmanager:CreateSecret",
						"secretsmanager:UpdateSecret", "secretsmanager:DeleteSecret",
						"secretsmanager:GetSecretValue", "secretsmanager:PutSecretValue",
						"secretsmanager:TagResource",
					},
					Resources: []string{"arn:aws:secretsmanager:*:123456789012:secret:teleport/*"},
				},
				{Effect: awslib.EffectAllow, Resources: []string{roleTarget.String()}, Actions: []string{
					"iam:GetRolePolicy", "iam:PutRolePolicy", "iam:DeleteRolePolicy",
				}},
			},
			boundaryStatements: []*awslib.Statement{
				{Effect: awslib.EffectAllow, Resources: []string{"*"}, Actions: []string{
					"elasticache:ListTagsForResource",
					"elasticache:DescribeReplicationGroups",
					"elasticache:DescribeCacheClusters",
					"elasticache:DescribeCacheSubnetGroups",
					"elasticache:DescribeUsers",
					"elasticache:ModifyUser",
					"elasticache:Connect",
				}},
				{
					Effect: awslib.EffectAllow,
					Actions: []string{
						"secretsmanager:DescribeSecret", "secretsmanager:CreateSecret",
						"secretsmanager:UpdateSecret", "secretsmanager:DeleteSecret",
						"secretsmanager:GetSecretValue", "secretsmanager:PutSecretValue",
						"secretsmanager:TagResource",
					},
					Resources: []string{"arn:aws:secretsmanager:*:123456789012:secret:teleport/*"},
				},
				{Effect: awslib.EffectAllow, Resources: []string{roleTarget.String()}, Actions: []string{
					"iam:GetRolePolicy", "iam:PutRolePolicy", "iam:DeleteRolePolicy",
				}},
			},
		},
		"ElastiCache static database": {
			target: roleTarget,
			fileConfig: &config.FileConfig{
				Databases: config.Databases{
					Service: config.Service{EnabledFlag: "true"},
					Databases: []*config.Database{
						{
							Name:     "redis-1",
							Protocol: "redis",
							URI:      "clustercfg.redis1.xxxxxx.usw2.cache.amazonaws.com:6379",
						},
						{
							Name:     "redis-2",
							Protocol: "redis",
							URI:      "clustercfg.redis2.xxxxxx.usw2.cache.amazonaws.com:6379",
							AWS: config.DatabaseAWS{
								SecretStore: config.SecretStore{
									KeyPrefix: "my-prefix/",
									KMSKeyID:  "my-kms-id",
								},
							},
						},
					},
				},
			},
			statements: []*awslib.Statement{
				{Effect: awslib.EffectAllow, Resources: []string{"*"}, Actions: []string{
					"elasticache:ListTagsForResource",
					"elasticache:DescribeReplicationGroups",
					"elasticache:DescribeCacheClusters",
					"elasticache:DescribeCacheSubnetGroups",
					"elasticache:DescribeUsers",
					"elasticache:ModifyUser",
				}},
				{
					Effect: "Allow",
					Actions: []string{
						"secretsmanager:DescribeSecret", "secretsmanager:CreateSecret",
						"secretsmanager:UpdateSecret", "secretsmanager:DeleteSecret",
						"secretsmanager:GetSecretValue", "secretsmanager:PutSecretValue",
						"secretsmanager:TagResource",
					},
					Resources: []string{
						"arn:aws:secretsmanager:*:123456789012:secret:teleport/*",
						"arn:aws:secretsmanager:*:123456789012:secret:my-prefix/*",
					},
				},
				{
					Effect:  "Allow",
					Actions: []string{"kms:GenerateDataKey", "kms:Decrypt"},
					Resources: []string{
						"arn:aws:kms:*:123456789012:key/my-kms-id",
					},
				},
				{Effect: awslib.EffectAllow, Resources: []string{roleTarget.String()}, Actions: []string{
					"iam:GetRolePolicy", "iam:PutRolePolicy", "iam:DeleteRolePolicy",
				}},
			},
			boundaryStatements: []*awslib.Statement{
				{Effect: awslib.EffectAllow, Resources: []string{"*"}, Actions: []string{
					"elasticache:ListTagsForResource",
					"elasticache:DescribeReplicationGroups",
					"elasticache:DescribeCacheClusters",
					"elasticache:DescribeCacheSubnetGroups",
					"elasticache:DescribeUsers",
					"elasticache:ModifyUser",
					"elasticache:Connect",
				}},
				{
					Effect: "Allow",
					Actions: []string{
						"secretsmanager:DescribeSecret", "secretsmanager:CreateSecret",
						"secretsmanager:UpdateSecret", "secretsmanager:DeleteSecret",
						"secretsmanager:GetSecretValue", "secretsmanager:PutSecretValue",
						"secretsmanager:TagResource",
					},
					Resources: []string{
						"arn:aws:secretsmanager:*:123456789012:secret:teleport/*",
						"arn:aws:secretsmanager:*:123456789012:secret:my-prefix/*",
					},
				},
				{
					Effect:  "Allow",
					Actions: []string{"kms:GenerateDataKey", "kms:Decrypt"},
					Resources: []string{
						"arn:aws:kms:*:123456789012:key/my-kms-id",
					},
				},
				{Effect: awslib.EffectAllow, Resources: []string{roleTarget.String()}, Actions: []string{
					"iam:GetRolePolicy", "iam:PutRolePolicy", "iam:DeleteRolePolicy",
				}},
			},
		},
		"MemoryDB auto discovery": {
			target: roleTarget,
			fileConfig: &config.FileConfig{
				Databases: config.Databases{
					Service: config.Service{EnabledFlag: "true"},
					AWSMatchers: []config.AWSMatcher{
						{Types: []string{services.AWSMatcherMemoryDB}, Regions: []string{"us-west-2"}},
					},
				},
			},
			statements: []*awslib.Statement{
				{Effect: awslib.EffectAllow, Resources: []string{"*"}, Actions: []string{
					"memorydb:ListTags",
					"memorydb:DescribeClusters",
					"memorydb:DescribeSubnetGroups",
					"memorydb:DescribeUsers",
					"memorydb:UpdateUser",
				}},
				{
					Effect: awslib.EffectAllow,
					Actions: []string{
						"secretsmanager:DescribeSecret", "secretsmanager:CreateSecret",
						"secretsmanager:UpdateSecret", "secretsmanager:DeleteSecret",
						"secretsmanager:GetSecretValue", "secretsmanager:PutSecretValue",
						"secretsmanager:TagResource",
					},
					Resources: []string{"arn:aws:secretsmanager:*:123456789012:secret:teleport/*"},
				},
			},
			boundaryStatements: []*awslib.Statement{
				{Effect: awslib.EffectAllow, Resources: []string{"*"}, Actions: []string{
					"memorydb:ListTags",
					"memorydb:DescribeClusters",
					"memorydb:DescribeSubnetGroups",
					"memorydb:DescribeUsers",
					"memorydb:UpdateUser",
				}},
				{
					Effect: awslib.EffectAllow,
					Actions: []string{
						"secretsmanager:DescribeSecret", "secretsmanager:CreateSecret",
						"secretsmanager:UpdateSecret", "secretsmanager:DeleteSecret",
						"secretsmanager:GetSecretValue", "secretsmanager:PutSecretValue",
						"secretsmanager:TagResource",
					},
					Resources: []string{"arn:aws:secretsmanager:*:123456789012:secret:teleport/*"},
				},
			},
		},
		"MemoryDB static database": {
			target: roleTarget,
			fileConfig: &config.FileConfig{
				Databases: config.Databases{
					Service: config.Service{EnabledFlag: "true"},
					Databases: []*config.Database{
						{
							Name:     "memorydb-1",
							Protocol: "redis",
							URI:      "clustercfg.my-memorydb-1.xxxxxx.memorydb.us-east-1.amazonaws.com:6379",
						},
						{
							Name:     "memorydb-2",
							Protocol: "redis",
							URI:      "clustercfg.my-memorydb-2.xxxxxx.memorydb.us-east-1.amazonaws.com:6379",
							AWS: config.DatabaseAWS{
								SecretStore: config.SecretStore{
									KeyPrefix: "my-prefix/",
									KMSKeyID:  "my-kms-id",
								},
							},
						},
					},
				},
			},
			statements: []*awslib.Statement{
				{Effect: awslib.EffectAllow, Resources: []string{"*"}, Actions: []string{
					"memorydb:ListTags",
					"memorydb:DescribeClusters",
					"memorydb:DescribeSubnetGroups",
					"memorydb:DescribeUsers",
					"memorydb:UpdateUser",
				}},
				{
					Effect: "Allow",
					Actions: []string{
						"secretsmanager:DescribeSecret", "secretsmanager:CreateSecret",
						"secretsmanager:UpdateSecret", "secretsmanager:DeleteSecret",
						"secretsmanager:GetSecretValue", "secretsmanager:PutSecretValue",
						"secretsmanager:TagResource",
					},
					Resources: []string{
						"arn:aws:secretsmanager:*:123456789012:secret:teleport/*",
						"arn:aws:secretsmanager:*:123456789012:secret:my-prefix/*",
					},
				},
				{
					Effect:  "Allow",
					Actions: []string{"kms:GenerateDataKey", "kms:Decrypt"},
					Resources: []string{
						"arn:aws:kms:*:123456789012:key/my-kms-id",
					},
				},
			},
			boundaryStatements: []*awslib.Statement{
				{Effect: awslib.EffectAllow, Resources: []string{"*"}, Actions: []string{
					"memorydb:ListTags",
					"memorydb:DescribeClusters",
					"memorydb:DescribeSubnetGroups",
					"memorydb:DescribeUsers",
					"memorydb:UpdateUser",
				}},
				{
					Effect: "Allow",
					Actions: []string{
						"secretsmanager:DescribeSecret", "secretsmanager:CreateSecret",
						"secretsmanager:UpdateSecret", "secretsmanager:DeleteSecret",
						"secretsmanager:GetSecretValue", "secretsmanager:PutSecretValue",
						"secretsmanager:TagResource",
					},
					Resources: []string{
						"arn:aws:secretsmanager:*:123456789012:secret:teleport/*",
						"arn:aws:secretsmanager:*:123456789012:secret:my-prefix/*",
					},
				},
				{
					Effect:  "Allow",
					Actions: []string{"kms:GenerateDataKey", "kms:Decrypt"},
					Resources: []string{
						"arn:aws:kms:*:123456789012:key/my-kms-id",
					},
				},
			},
		},
		"AutoDiscovery EC2": {
			target: roleTarget,
			flags:  configurators.BootstrapFlags{Service: configurators.DiscoveryService},
			fileConfig: &config.FileConfig{
				Discovery: config.Discovery{
					Service: config.Service{EnabledFlag: "true"},
					AWSMatchers: []config.AWSMatcher{
						{
							Types:   []string{services.AWSMatcherEC2},
							Regions: []string{"eu-central-1"},
							Tags:    map[string]utils.Strings{"*": []string{"*"}},
							InstallParams: &config.InstallParams{
								JoinParams: config.JoinParams{
									TokenName: "token",
									Method:    types.JoinMethodIAM,
								},
							},
						},
					},
				},
			},
			statements: []*awslib.Statement{
				{
					Effect: "Allow",
					Actions: []string{
						"ec2:DescribeInstances",
						"ssm:GetCommandInvocation",
						"ssm:SendCommand"},
					Resources: []string{"*"},
				},
			},
			boundaryStatements: []*awslib.Statement{
				{
					Effect: "Allow",
					Actions: []string{
						"ec2:DescribeInstances",
						"ssm:GetCommandInvocation",
						"ssm:SendCommand"},
					Resources: []string{"*"},
				},
			},
		},
		"AutoDiscoveryUnknownIdentity": {
			returnError: true,
			target:      unknownIdentity,
			fileConfig: &config.FileConfig{
				Databases: config.Databases{
					Service: config.Service{EnabledFlag: "true"},
					AWSMatchers: []config.AWSMatcher{
						{Types: []string{services.AWSMatcherRDS}, Regions: []string{"us-west-2"}},
					},
				},
			},
		},
		"RDS Proxy discovery": {
			target: userTarget,
			fileConfig: &config.FileConfig{
				Databases: config.Databases{
					Service: config.Service{EnabledFlag: "true"},
					AWSMatchers: []config.AWSMatcher{
						{Types: []string{services.AWSMatcherRDSProxy}, Regions: []string{"us-west-2"}},
					},
				},
			},
			statements: []*awslib.Statement{
				{Effect: awslib.EffectAllow, Resources: []string{"*"}, Actions: []string{
					"rds:DescribeDBProxies", "rds:DescribeDBProxyEndpoints", "rds:DescribeDBProxyTargets", "rds:ListTagsForResource",
				}},
				{Effect: awslib.EffectAllow, Resources: []string{userTarget.String()}, Actions: []string{
					"iam:GetUserPolicy", "iam:PutUserPolicy", "iam:DeleteUserPolicy",
				}},
			},
			boundaryStatements: []*awslib.Statement{
				{Effect: awslib.EffectAllow, Resources: []string{"*"}, Actions: []string{
					"rds:DescribeDBProxies", "rds:DescribeDBProxyEndpoints", "rds:DescribeDBProxyTargets", "rds:ListTagsForResource",
					"rds-db:connect",
				}},
				{Effect: awslib.EffectAllow, Resources: []string{userTarget.String()}, Actions: []string{
					"iam:GetUserPolicy", "iam:PutUserPolicy", "iam:DeleteUserPolicy",
				}},
			},
		},
		"RDS Proxy static database": {
			target: userTarget,
			fileConfig: &config.FileConfig{
				Databases: config.Databases{
					Service: config.Service{EnabledFlag: "true"},
					Databases: []*config.Database{
						{
							Name:     "rds-proxy-1",
							Protocol: "postgres",
							URI:      "my-proxy.proxy-abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
						},
					},
				},
			},
			statements: []*awslib.Statement{
				{Effect: awslib.EffectAllow, Resources: []string{"*"}, Actions: []string{
					"rds:DescribeDBProxies", "rds:DescribeDBProxyEndpoints", "rds:DescribeDBProxyTargets", "rds:ListTagsForResource",
				}},
				{Effect: awslib.EffectAllow, Resources: []string{userTarget.String()}, Actions: []string{
					"iam:GetUserPolicy", "iam:PutUserPolicy", "iam:DeleteUserPolicy",
				}},
			},
			boundaryStatements: []*awslib.Statement{
				{Effect: awslib.EffectAllow, Resources: []string{"*"}, Actions: []string{
					"rds:DescribeDBProxies", "rds:DescribeDBProxyEndpoints", "rds:DescribeDBProxyTargets", "rds:ListTagsForResource",
					"rds-db:connect",
				}},
				{Effect: awslib.EffectAllow, Resources: []string{userTarget.String()}, Actions: []string{
					"iam:GetUserPolicy", "iam:PutUserPolicy", "iam:DeleteUserPolicy",
				}},
			},
		},
		"Redshift Serverless discovery": {
			target: roleTarget,
			fileConfig: &config.FileConfig{
				Databases: config.Databases{
					Service: config.Service{EnabledFlag: "true"},
					AWSMatchers: []config.AWSMatcher{
						{Types: []string{services.AWSMatcherRedshiftServerless}, Regions: []string{"us-west-2"}},
					},
				},
			},
			statements: []*awslib.Statement{
				{
					Effect:    awslib.EffectAllow,
					Resources: awslib.SliceOrString{"*"},
					Actions:   awslib.SliceOrString{"redshift-serverless:GetEndpointAccess", "redshift-serverless:GetWorkgroup", "redshift-serverless:ListWorkgroups", "redshift-serverless:ListEndpointAccess", "redshift-serverless:ListTagsForResource"},
				},
			},
			boundaryStatements: []*awslib.Statement{
				{
					Effect:    awslib.EffectAllow,
					Resources: awslib.SliceOrString{"*"},
					Actions:   awslib.SliceOrString{"redshift-serverless:GetEndpointAccess", "redshift-serverless:GetWorkgroup", "redshift-serverless:ListWorkgroups", "redshift-serverless:ListEndpointAccess", "redshift-serverless:ListTagsForResource", "sts:AssumeRole"},
				},
			},
		},
		"Redshift Serverless static database": {
			target: roleTarget,
			fileConfig: &config.FileConfig{
				Databases: config.Databases{
					Service: config.Service{EnabledFlag: "true"},
					Databases: []*config.Database{
						{
							Name:     "redshift-serverless-1",
							Protocol: "postgres",
							URI:      fmt.Sprintf("%s:5439", aws.StringValue(mocks.RedshiftServerlessWorkgroup("redshift-serverless-1", "us-west-2").Endpoint.Address)),
						},
					},
				},
			},
			statements: []*awslib.Statement{
				{
					Effect:    awslib.EffectAllow,
					Resources: awslib.SliceOrString{"*"},
					Actions:   awslib.SliceOrString{"redshift-serverless:GetEndpointAccess", "redshift-serverless:GetWorkgroup", "redshift-serverless:ListWorkgroups", "redshift-serverless:ListEndpointAccess", "redshift-serverless:ListTagsForResource"},
				},
			},
			boundaryStatements: []*awslib.Statement{
				{
					Effect:    awslib.EffectAllow,
					Resources: awslib.SliceOrString{"*"},
					Actions:   awslib.SliceOrString{"redshift-serverless:GetEndpointAccess", "redshift-serverless:GetWorkgroup", "redshift-serverless:ListWorkgroups", "redshift-serverless:ListEndpointAccess", "redshift-serverless:ListTagsForResource", "sts:AssumeRole"},
				},
			},
		},
		"AWS Keyspaces static databases": {
			target: roleTarget,
			fileConfig: &config.FileConfig{
				Databases: config.Databases{
					Service: config.Service{EnabledFlag: "true"},
					Databases: []*config.Database{{
						Name:     "keyspaces-1",
						Protocol: "cassandra",
						AWS: config.DatabaseAWS{
							AccountID: "123456789012",
							Region:    "us-west-1",
						},
					}},
				},
			},
			boundaryStatements: []*awslib.Statement{
				{
					Effect:    awslib.EffectAllow,
					Resources: awslib.SliceOrString{"*"},
					Actions:   awslib.SliceOrString{"sts:AssumeRole"},
				},
			},
		},
		"DynamoDB static databases": {
			target: roleTarget,
			fileConfig: &config.FileConfig{
				Databases: config.Databases{
					Service: config.Service{EnabledFlag: "true"},
					Databases: []*config.Database{{
						Name:     "dynamodb-1",
						Protocol: "dynamodb",
						AWS: config.DatabaseAWS{
							Region:    "us-west-1",
							AccountID: "123456789012",
						},
					}},
				},
			},
			boundaryStatements: []*awslib.Statement{
				{
					Effect:    awslib.EffectAllow,
					Resources: awslib.SliceOrString{"*"},
					Actions:   awslib.SliceOrString{"sts:AssumeRole"},
				},
			},
		},
		"target in assume role": {
			target: roleTarget,
			fileConfig: &config.FileConfig{
				Databases: config.Databases{
					Service: config.Service{EnabledFlag: "true"},
					Databases: []*config.Database{
						{
							Name:     "aurora-2",
							Protocol: "postgres",
							URI:      "aurora-instance-2.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
							AWS: config.DatabaseAWS{
								AssumeRoleARN: role2,
							},
						},
						{
							Name:     "aurora-1",
							Protocol: "postgres",
							URI:      "aurora-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
							AWS: config.DatabaseAWS{
								AssumeRoleARN: role3,
								ExternalID:    "someID",
							},
						},
					},
					AWSMatchers: []config.AWSMatcher{
						{Types: []string{services.AWSMatcherRDS}, Regions: []string{"us-west-2"}, AssumeRoleARN: role4},
						{Types: []string{services.AWSMatcherRDSProxy}, Regions: []string{"us-west-2"}, AssumeRoleARN: roleTarget.String(), ExternalID: "foo"},
					},
				},
			},
			statements: []*awslib.Statement{
				{Effect: awslib.EffectAllow, Resources: []string{"*"}, Actions: []string{
					"rds:DescribeDBProxies", "rds:DescribeDBProxyEndpoints", "rds:DescribeDBProxyTargets", "rds:ListTagsForResource",
				}},
				{Effect: awslib.EffectAllow, Resources: []string{roleTarget.String()}, Actions: []string{
					"iam:GetRolePolicy", "iam:PutRolePolicy", "iam:DeleteRolePolicy",
				}},
			},
			boundaryStatements: []*awslib.Statement{
				{Effect: awslib.EffectAllow, Resources: []string{"*"}, Actions: []string{
					"rds:DescribeDBProxies", "rds:DescribeDBProxyEndpoints", "rds:DescribeDBProxyTargets", "rds:ListTagsForResource",
					"rds-db:connect",
				}},
				{Effect: awslib.EffectAllow, Resources: []string{roleTarget.String()}, Actions: []string{
					"iam:GetRolePolicy", "iam:PutRolePolicy", "iam:DeleteRolePolicy",
				}},
			},
		},
		"target not in assume role": {
			target: roleTarget,
			flags:  configurators.BootstrapFlags{ForceAssumesRoles: role1},
			fileConfig: &config.FileConfig{
				Databases: config.Databases{
					Service: config.Service{EnabledFlag: "true"},
					Databases: []*config.Database{
						{
							Name:     "aurora-2",
							Protocol: "postgres",
							URI:      "aurora-instance-2.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
							AWS: config.DatabaseAWS{
								AssumeRoleARN: role2,
							},
						},
						{
							Name:     "aurora-1",
							Protocol: "postgres",
							URI:      "aurora-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
							AWS: config.DatabaseAWS{
								AssumeRoleARN: role3,
								ExternalID:    "someID",
							},
						},
					},
					AWSMatchers: []config.AWSMatcher{
						{Types: []string{services.AWSMatcherRDS}, Regions: []string{"us-west-2"}, AssumeRoleARN: role4},
						{Types: []string{services.AWSMatcherRDS}, Regions: []string{"us-west-2"}, AssumeRoleARN: role5, ExternalID: "foo"},
					},
				},
			},
			statements: []*awslib.Statement{
				{
					Effect:    awslib.EffectAllow,
					Resources: awslib.SliceOrString{role1, role2, role3, role4, role5},
					Actions:   awslib.SliceOrString{"sts:AssumeRole"},
				},
			},
			boundaryStatements: []*awslib.Statement{{
				Effect:    awslib.EffectAllow,
				Resources: []string{"*"},
				Actions: []string{
					"sts:AssumeRole",
				}},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			targetCfg := mustGetTargetConfig(t, test.flags, test.target, test.fileConfig)
			policy, policyErr := buildPolicyDocument(test.flags, targetCfg, false)
			boundary, boundaryErr := buildPolicyDocument(test.flags, targetCfg, true)

			if test.returnError {
				require.Error(t, policyErr)
				require.Error(t, boundaryErr)
				return
			}

			require.NoError(t, policyErr)
			require.NoError(t, boundaryErr)
			require.Empty(t, cmp.Diff(test.statements, policy.Document.Statements, sortStringsTrans))
			require.Empty(t, cmp.Diff(test.boundaryStatements, boundary.Document.Statements, sortStringsTrans))
		})
	}

	t.Run("discovery service", func(t *testing.T) {
		tests := map[string]struct {
			fileConfig           *config.FileConfig
			statements           []*awslib.Statement
			boundaryStatements   []*awslib.Statement
			wantInlineAsBoundary bool
		}{
			"RDS": {
				fileConfig: &config.FileConfig{
					Discovery: config.Discovery{
						Service: config.Service{EnabledFlag: "true"},
						AWSMatchers: []config.AWSMatcher{
							{Types: []string{services.AWSMatcherRDS}, Regions: []string{"us-west-2"}},
						},
					},
				},
				statements: []*awslib.Statement{
					{
						Effect:    awslib.EffectAllow,
						Resources: awslib.SliceOrString{"*"},
						Actions:   awslib.SliceOrString{"rds:DescribeDBInstances", "rds:DescribeDBClusters"},
					},
				},
				wantInlineAsBoundary: true,
			},
			"RDS Proxy": {
				fileConfig: &config.FileConfig{
					Discovery: config.Discovery{
						Service: config.Service{EnabledFlag: "true"},
						AWSMatchers: []config.AWSMatcher{
							{Types: []string{services.AWSMatcherRDSProxy}, Regions: []string{"us-west-2"}},
						},
					},
				},
				statements: []*awslib.Statement{
					{
						Effect:    awslib.EffectAllow,
						Resources: awslib.SliceOrString{"*"},
						Actions:   awslib.SliceOrString{"rds:DescribeDBProxies", "rds:DescribeDBProxyEndpoints", "rds:DescribeDBProxyTargets", "rds:ListTagsForResource"},
					},
				},
				wantInlineAsBoundary: true,
			},
			"Redshift": {
				fileConfig: &config.FileConfig{
					Discovery: config.Discovery{
						Service: config.Service{EnabledFlag: "true"},
						AWSMatchers: []config.AWSMatcher{
							{Types: []string{services.AWSMatcherRedshift}, Regions: []string{"us-west-2"}},
						},
					},
				},
				statements: []*awslib.Statement{
					{
						Effect:    awslib.EffectAllow,
						Resources: awslib.SliceOrString{"*"},
						Actions:   awslib.SliceOrString{"redshift:DescribeClusters"},
					},
				},
				wantInlineAsBoundary: true,
			},
			"Redshift Serverless": {
				fileConfig: &config.FileConfig{
					Discovery: config.Discovery{
						Service: config.Service{EnabledFlag: "true"},
						AWSMatchers: []config.AWSMatcher{
							{Types: []string{services.AWSMatcherRedshiftServerless}, Regions: []string{"us-west-2"}},
						},
					},
				},
				statements: []*awslib.Statement{
					{
						Effect:    awslib.EffectAllow,
						Resources: awslib.SliceOrString{"*"},
						Actions:   awslib.SliceOrString{"redshift-serverless:ListWorkgroups", "redshift-serverless:ListEndpointAccess", "redshift-serverless:ListTagsForResource"},
					},
				},
				wantInlineAsBoundary: true,
			},
			"ElastiCache": {
				fileConfig: &config.FileConfig{
					Discovery: config.Discovery{
						Service: config.Service{EnabledFlag: "true"},
						AWSMatchers: []config.AWSMatcher{
							{Types: []string{services.AWSMatcherElastiCache}, Regions: []string{"us-west-2"}},
						},
					},
				},
				statements: []*awslib.Statement{
					{
						Effect:    awslib.EffectAllow,
						Resources: awslib.SliceOrString{"*"},
						Actions:   awslib.SliceOrString{"elasticache:ListTagsForResource", "elasticache:DescribeReplicationGroups", "elasticache:DescribeCacheClusters", "elasticache:DescribeCacheSubnetGroups"},
					},
				},
				wantInlineAsBoundary: true,
			},
			"MemoryDB": {
				fileConfig: &config.FileConfig{
					Discovery: config.Discovery{
						Service: config.Service{EnabledFlag: "true"},
						AWSMatchers: []config.AWSMatcher{
							{Types: []string{services.AWSMatcherMemoryDB}, Regions: []string{"us-west-2"}},
						},
					},
				},
				statements: []*awslib.Statement{
					{
						Effect:    awslib.EffectAllow,
						Resources: awslib.SliceOrString{"*"},
						Actions:   awslib.SliceOrString{"memorydb:ListTags", "memorydb:DescribeClusters", "memorydb:DescribeSubnetGroups"},
					},
				},
				wantInlineAsBoundary: true,
			},
			"multiple": {
				fileConfig: &config.FileConfig{
					Discovery: config.Discovery{
						Service: config.Service{EnabledFlag: "true"},
						AWSMatchers: []config.AWSMatcher{
							{Types: []string{services.AWSMatcherRedshift}, Regions: []string{"us-west-1"}},
							{Types: []string{services.AWSMatcherRedshift, services.AWSMatcherRDS, services.AWSMatcherRDSProxy}, Regions: []string{"us-west-2"}},
							{Types: []string{services.AWSMatcherElastiCache}, Regions: []string{"us-west-2"}, AssumeRoleARN: role1, ExternalID: "foo"},
						},
					},
				},
				statements: []*awslib.Statement{
					{
						Effect:    awslib.EffectAllow,
						Resources: awslib.SliceOrString{"*"},
						Actions:   awslib.SliceOrString{"rds:DescribeDBInstances", "rds:DescribeDBClusters"},
					},
					{
						Effect:    awslib.EffectAllow,
						Resources: awslib.SliceOrString{"*"},
						Actions:   awslib.SliceOrString{"rds:DescribeDBProxies", "rds:DescribeDBProxyEndpoints", "rds:DescribeDBProxyTargets", "rds:ListTagsForResource"},
					},
					{
						Effect:    awslib.EffectAllow,
						Resources: awslib.SliceOrString{"*"},
						Actions:   awslib.SliceOrString{"redshift:DescribeClusters"},
					},
					{
						Effect:    awslib.EffectAllow,
						Resources: awslib.SliceOrString{role1},
						Actions:   awslib.SliceOrString{"sts:AssumeRole"},
					},
				},
				boundaryStatements: []*awslib.Statement{
					{
						Effect:    awslib.EffectAllow,
						Resources: awslib.SliceOrString{"*"},
						Actions:   awslib.SliceOrString{"rds:DescribeDBInstances", "rds:DescribeDBClusters"},
					},
					{
						Effect:    awslib.EffectAllow,
						Resources: awslib.SliceOrString{"*"},
						Actions:   awslib.SliceOrString{"rds:DescribeDBProxies", "rds:DescribeDBProxyEndpoints", "rds:DescribeDBProxyTargets", "rds:ListTagsForResource"},
					},
					{
						Effect:    awslib.EffectAllow,
						Resources: awslib.SliceOrString{"*"},
						Actions:   awslib.SliceOrString{"redshift:DescribeClusters"},
					},
					{
						Effect:    awslib.EffectAllow,
						Resources: awslib.SliceOrString{"*"},
						Actions:   awslib.SliceOrString{"sts:AssumeRole"},
					},
				},
				wantInlineAsBoundary: false,
			},
		}

		flags := configurators.BootstrapFlags{Service: configurators.DiscoveryService}
		for name, test := range tests {
			t.Run(name, func(t *testing.T) {
				targetCfg := mustGetTargetConfig(t, flags, roleTarget, test.fileConfig)

				if test.wantInlineAsBoundary {
					test.boundaryStatements = test.statements
				}

				mustBuildPolicyDocument(t, flags, targetCfg, false, test.statements)
				mustBuildPolicyDocument(t, flags, targetCfg, true, test.boundaryStatements)
			})
		}
	})

	t.Run("database service with discovery service config", func(t *testing.T) {
		tests := map[string]struct {
			fileConfig         *config.FileConfig
			statements         []*awslib.Statement
			boundaryStatements []*awslib.Statement
		}{
			"RDS": {
				fileConfig: &config.FileConfig{
					Discovery: config.Discovery{
						Service: config.Service{EnabledFlag: "true"},
						AWSMatchers: []config.AWSMatcher{
							{Types: []string{services.AWSMatcherRDS}, Regions: []string{"us-west-2"}},
						},
					},
				},
				statements: []*awslib.Statement{
					{
						Effect:    awslib.EffectAllow,
						Resources: []string{"*"},
						Actions:   []string{"rds:ModifyDBInstance", "rds:ModifyDBCluster"},
					},
					{
						Effect:    awslib.EffectAllow,
						Resources: []string{roleTarget.String()},
						Actions:   []string{"iam:GetRolePolicy", "iam:PutRolePolicy", "iam:DeleteRolePolicy"},
					},
				},
				boundaryStatements: []*awslib.Statement{
					{
						Effect:    awslib.EffectAllow,
						Resources: []string{"*"},
						Actions:   []string{"rds:ModifyDBInstance", "rds:ModifyDBCluster", "rds-db:connect"},
					},
					{
						Effect:    awslib.EffectAllow,
						Resources: []string{roleTarget.String()},
						Actions:   []string{"iam:GetRolePolicy", "iam:PutRolePolicy", "iam:DeleteRolePolicy"},
					},
				},
			},
			"RDS Proxy": {
				fileConfig: &config.FileConfig{
					Discovery: config.Discovery{
						Service: config.Service{EnabledFlag: "true"},
						AWSMatchers: []config.AWSMatcher{
							{Types: []string{services.AWSMatcherRDSProxy}, Regions: []string{"us-west-2"}},
						},
					},
				},
				statements: []*awslib.Statement{
					{
						Effect:    awslib.EffectAllow,
						Resources: []string{roleTarget.String()},
						Actions:   []string{"iam:GetRolePolicy", "iam:PutRolePolicy", "iam:DeleteRolePolicy"},
					},
				},
				boundaryStatements: []*awslib.Statement{
					{
						Effect:    awslib.EffectAllow,
						Resources: []string{"*"},
						Actions:   []string{"rds-db:connect"},
					},
					{
						Effect:    awslib.EffectAllow,
						Resources: []string{roleTarget.String()},
						Actions:   []string{"iam:GetRolePolicy", "iam:PutRolePolicy", "iam:DeleteRolePolicy"},
					},
				},
			},
			"Redshift": {
				fileConfig: &config.FileConfig{
					Discovery: config.Discovery{
						Service: config.Service{EnabledFlag: "true"},
						AWSMatchers: []config.AWSMatcher{
							{Types: []string{services.AWSMatcherRedshift}, Regions: []string{"us-west-2"}},
						},
					},
				},
				statements: []*awslib.Statement{
					{
						Effect:    awslib.EffectAllow,
						Resources: []string{roleTarget.String()},
						Actions:   []string{"iam:GetRolePolicy", "iam:PutRolePolicy", "iam:DeleteRolePolicy"},
					},
				},
				boundaryStatements: []*awslib.Statement{
					{
						Effect:    awslib.EffectAllow,
						Resources: []string{"*"},
						Actions:   []string{"redshift:GetClusterCredentials"},
					},
					{
						Effect:    awslib.EffectAllow,
						Resources: []string{roleTarget.String()},
						Actions:   []string{"iam:GetRolePolicy", "iam:PutRolePolicy", "iam:DeleteRolePolicy"},
					},
				},
			},
			"Redshift Serverless": {
				fileConfig: &config.FileConfig{
					Discovery: config.Discovery{
						Service: config.Service{EnabledFlag: "true"},
						AWSMatchers: []config.AWSMatcher{
							{Types: []string{services.AWSMatcherRedshiftServerless}, Regions: []string{"us-west-2"}},
						},
					},
				},
				boundaryStatements: []*awslib.Statement{
					{
						Effect:    awslib.EffectAllow,
						Resources: []string{"*"},
						Actions:   []string{"sts:AssumeRole"},
					},
				},
			},
			"ElastiCache": {
				fileConfig: &config.FileConfig{
					Discovery: config.Discovery{
						Service: config.Service{EnabledFlag: "true"},
						AWSMatchers: []config.AWSMatcher{
							{Types: []string{services.AWSMatcherElastiCache}, Regions: []string{"us-west-2"}},
						},
					},
				},
				statements: []*awslib.Statement{
					{
						Effect:    awslib.EffectAllow,
						Resources: []string{"*"},
						Actions:   []string{"elasticache:DescribeUsers", "elasticache:ModifyUser"},
					},
					{
						Effect: awslib.EffectAllow,
						Actions: []string{
							"secretsmanager:DescribeSecret", "secretsmanager:CreateSecret",
							"secretsmanager:UpdateSecret", "secretsmanager:DeleteSecret",
							"secretsmanager:GetSecretValue", "secretsmanager:PutSecretValue",
							"secretsmanager:TagResource",
						},
						Resources: []string{"arn:aws:secretsmanager:*:123456789012:secret:teleport/*"},
					},
					{
						Effect:    awslib.EffectAllow,
						Resources: []string{roleTarget.String()},
						Actions:   []string{"iam:GetRolePolicy", "iam:PutRolePolicy", "iam:DeleteRolePolicy"},
					},
				},
				boundaryStatements: []*awslib.Statement{
					{
						Effect:    awslib.EffectAllow,
						Resources: []string{"*"},
						Actions:   []string{"elasticache:DescribeUsers", "elasticache:ModifyUser", "elasticache:Connect"},
					},
					{
						Effect: awslib.EffectAllow,
						Actions: []string{
							"secretsmanager:DescribeSecret", "secretsmanager:CreateSecret",
							"secretsmanager:UpdateSecret", "secretsmanager:DeleteSecret",
							"secretsmanager:GetSecretValue", "secretsmanager:PutSecretValue",
							"secretsmanager:TagResource",
						},
						Resources: []string{"arn:aws:secretsmanager:*:123456789012:secret:teleport/*"},
					},
					{
						Effect:    awslib.EffectAllow,
						Resources: []string{roleTarget.String()},
						Actions:   []string{"iam:GetRolePolicy", "iam:PutRolePolicy", "iam:DeleteRolePolicy"},
					},
				},
			},
			"MemoryDB": {
				fileConfig: &config.FileConfig{
					Discovery: config.Discovery{
						Service: config.Service{EnabledFlag: "true"},
						AWSMatchers: []config.AWSMatcher{
							{Types: []string{services.AWSMatcherMemoryDB}, Regions: []string{"us-west-2"}},
						},
					},
				},
				statements: []*awslib.Statement{
					{
						Effect:    awslib.EffectAllow,
						Resources: []string{"*"},
						Actions:   []string{"memorydb:DescribeUsers", "memorydb:UpdateUser"},
					},
					{
						Effect: awslib.EffectAllow,
						Actions: []string{
							"secretsmanager:DescribeSecret", "secretsmanager:CreateSecret",
							"secretsmanager:UpdateSecret", "secretsmanager:DeleteSecret",
							"secretsmanager:GetSecretValue", "secretsmanager:PutSecretValue",
							"secretsmanager:TagResource",
						},
						Resources: []string{"arn:aws:secretsmanager:*:123456789012:secret:teleport/*"},
					},
				},
				boundaryStatements: []*awslib.Statement{
					{
						Effect:    awslib.EffectAllow,
						Resources: []string{"*"},
						Actions:   []string{"memorydb:DescribeUsers", "memorydb:UpdateUser"},
					},
					{
						Effect: awslib.EffectAllow,
						Actions: []string{
							"secretsmanager:DescribeSecret", "secretsmanager:CreateSecret",
							"secretsmanager:UpdateSecret", "secretsmanager:DeleteSecret",
							"secretsmanager:GetSecretValue", "secretsmanager:PutSecretValue",
							"secretsmanager:TagResource",
						},
						Resources: []string{"arn:aws:secretsmanager:*:123456789012:secret:teleport/*"},
					},
				},
			},
			"multiple": {
				fileConfig: &config.FileConfig{
					Discovery: config.Discovery{
						Service: config.Service{EnabledFlag: "true"},
						AWSMatchers: []config.AWSMatcher{
							{Types: []string{services.AWSMatcherRedshift}, Regions: []string{"us-west-1"}},
							{Types: []string{services.AWSMatcherRedshift, services.AWSMatcherRDS, services.AWSMatcherRDSProxy}, Regions: []string{"us-west-2"}},
							{Types: []string{services.AWSMatcherElastiCache}, Regions: []string{"us-west-2"}, AssumeRoleARN: role1, ExternalID: "foo"},
						},
					},
				},
				statements: []*awslib.Statement{
					{
						Effect:    awslib.EffectAllow,
						Resources: []string{"*"},
						Actions:   []string{"rds:ModifyDBInstance", "rds:ModifyDBCluster"},
					},
					{
						Effect:    awslib.EffectAllow,
						Resources: awslib.SliceOrString{role1},
						Actions:   awslib.SliceOrString{"sts:AssumeRole"},
					},
					{
						Effect:    awslib.EffectAllow,
						Resources: []string{roleTarget.String()},
						Actions:   []string{"iam:GetRolePolicy", "iam:PutRolePolicy", "iam:DeleteRolePolicy"},
					},
				},
				boundaryStatements: []*awslib.Statement{
					{
						Effect:    awslib.EffectAllow,
						Resources: []string{"*"},
						Actions:   []string{"rds:ModifyDBInstance", "rds:ModifyDBCluster", "rds-db:connect"},
					},
					{
						Effect:    awslib.EffectAllow,
						Resources: []string{"*"},
						Actions:   []string{"redshift:GetClusterCredentials"},
					},
					{
						Effect:    awslib.EffectAllow,
						Resources: awslib.SliceOrString{"*"},
						Actions:   awslib.SliceOrString{"sts:AssumeRole"},
					},
					{
						Effect:    awslib.EffectAllow,
						Resources: []string{roleTarget.String()},
						Actions:   []string{"iam:GetRolePolicy", "iam:PutRolePolicy", "iam:DeleteRolePolicy"},
					},
				},
			},
		}

		flags := configurators.BootstrapFlags{
			Service: configurators.DatabaseServiceByDiscoveryServiceConfig,
		}
		for name, test := range tests {
			t.Run(name, func(t *testing.T) {
				targetCfg := mustGetTargetConfig(t, flags, roleTarget, test.fileConfig)
				mustBuildPolicyDocument(t, flags, targetCfg, false, test.statements)
				mustBuildPolicyDocument(t, flags, targetCfg, true, test.boundaryStatements)
			})
		}
	})
}

func mustGetTargetConfig(t *testing.T, flags configurators.BootstrapFlags, roleTarget awslib.Identity, fileConfig *config.FileConfig) targetConfig {
	t.Helper()

	require.NoError(t, fileConfig.CheckAndSetDefaults())
	serviceCfg := servicecfg.MakeDefaultConfig()
	require.NoError(t, config.ApplyFileConfig(fileConfig, serviceCfg))
	targetCfg, err := getTargetConfig(flags, serviceCfg, roleTarget)
	require.NoError(t, err)
	return targetCfg
}

func mustBuildPolicyDocument(t *testing.T, flags configurators.BootstrapFlags, targetCfg targetConfig, boundary bool, wantStatements []*awslib.Statement) {
	t.Helper()

	policy, policyErr := buildPolicyDocument(flags, targetCfg, boundary)
	require.NoError(t, policyErr)
	require.Empty(t, cmp.Diff(wantStatements, policy.Document.Statements, sortStringsTrans))
}

func TestAWSPolicyCreator(t *testing.T) {
	ctx := context.Background()

	tests := map[string]struct {
		returnError bool
		isBoundary  bool
		policies    *policiesMock
	}{
		"UpsertPolicy": {
			policies: &policiesMock{
				upsertArn: "generated-arn",
			},
		},
		"UpsertPolicyBoundary": {
			isBoundary: true,
			policies: &policiesMock{
				upsertArn: "generated-arn",
			},
		},
		"UpsertPolicyFailure": {
			returnError: true,
			policies: &policiesMock{
				upsertError: trace.NotImplemented("upsert not implemented"),
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			action := &awsPolicyCreator{
				policies:   test.policies,
				isBoundary: test.isBoundary,
			}

			actionCtx := &configurators.ConfiguratorActionContext{}
			err := action.Execute(ctx, actionCtx)
			if test.returnError {
				require.Error(t, err)
				return
			}

			if test.isBoundary {
				require.Equal(t, test.policies.upsertArn, actionCtx.AWSPolicyBoundaryArn)
				return
			}

			require.Equal(t, test.policies.upsertArn, actionCtx.AWSPolicyArn)
		})
	}
}

func TestAWSPoliciesAttacher(t *testing.T) {
	ctx := context.Background()
	userTarget, err := awslib.IdentityFromArn("arn:aws:iam::1234567:user/example-user")
	require.NoError(t, err)

	roleTarget, err := awslib.IdentityFromArn("arn:aws:iam::1234567:role/example-role")
	require.NoError(t, err)

	tests := map[string]struct {
		returnError bool
		target      awslib.Identity
		policies    *policiesMock
		actionCtx   *configurators.ConfiguratorActionContext
	}{
		"AttachPoliciesToUser": {
			target:   userTarget,
			policies: &policiesMock{},
			actionCtx: &configurators.ConfiguratorActionContext{
				AWSPolicyArn:         "policy-arn",
				AWSPolicyBoundaryArn: "policy-boundary-arn",
			},
		},
		"AttachPoliciesToRole": {
			target:   roleTarget,
			policies: &policiesMock{},
			actionCtx: &configurators.ConfiguratorActionContext{
				AWSPolicyArn:         "policy-arn",
				AWSPolicyBoundaryArn: "policy-boundary-arn",
			},
		},
		"MissingPolicyArn": {
			returnError: true,
			target:      roleTarget,
			policies:    &policiesMock{},
			actionCtx: &configurators.ConfiguratorActionContext{
				AWSPolicyBoundaryArn: "policy-boundary-arn",
			},
		},
		"MissingPolicyBoundaryArn": {
			returnError: true,
			target:      roleTarget,
			policies:    &policiesMock{},
			actionCtx: &configurators.ConfiguratorActionContext{
				AWSPolicyArn: "policy-arn",
			},
		},
		"AttachPolicyFailure": {
			returnError: true,
			target:      roleTarget,
			policies: &policiesMock{
				attachError: trace.NotImplemented("attach policy not implemented"),
			},
			actionCtx: &configurators.ConfiguratorActionContext{
				AWSPolicyArn:         "policy-arn",
				AWSPolicyBoundaryArn: "policy-boundary-arn",
			},
		},
		"AttachPolicyBoundaryFailure": {
			returnError: true,
			target:      roleTarget,
			policies: &policiesMock{
				attachBoundaryError: trace.NotImplemented("attach policy not implemented"),
			},
			actionCtx: &configurators.ConfiguratorActionContext{
				AWSPolicyArn:         "policy-arn",
				AWSPolicyBoundaryArn: "policy-boundary-arn",
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			action := &awsPoliciesAttacher{policies: test.policies, target: test.target}
			err := action.Execute(ctx, test.actionCtx)
			if test.returnError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestAWSPoliciesTarget(t *testing.T) {
	userIdentity, err := awslib.IdentityFromArn("arn:aws:iam::123456789012:user/example-user")
	require.NoError(t, err)

	roleIdentity, err := awslib.IdentityFromArn("arn:aws:iam::123456789012:role/example-role")
	require.NoError(t, err)

	assumedRoleIdentity, err := awslib.IdentityFromArn("arn:aws:sts::123456789012:assumed-role/example-role/i-12345")
	require.NoError(t, err)

	tests := map[string]struct {
		flags             configurators.BootstrapFlags
		identity          awslib.Identity
		accountID         string
		partitionID       string
		targetType        awslib.Identity
		targetName        string
		targetAccountID   string
		targetPartitionID string
		targetString      string
		iamClient         iamiface.IAMAPI
		wantErrContains   string
	}{
		"UserNameFromFlags": {
			flags:             configurators.BootstrapFlags{AttachToUser: "example-user"},
			accountID:         "123456",
			partitionID:       "aws",
			targetType:        awslib.User{},
			targetName:        "example-user",
			targetAccountID:   "123456",
			targetPartitionID: "aws",
			targetString:      "arn:aws:iam::123456:user/example-user",
		},
		"UserNameWithPathFromFlags": {
			flags:             configurators.BootstrapFlags{AttachToUser: "/some/path/example-user"},
			accountID:         "123456",
			partitionID:       "aws",
			targetType:        awslib.User{},
			targetName:        "example-user",
			targetAccountID:   "123456",
			targetPartitionID: "aws",
			targetString:      "arn:aws:iam::123456:user/some/path/example-user",
		},
		"UserARNFromFlags": {
			flags:             configurators.BootstrapFlags{AttachToUser: "arn:aws:iam::123456789012:user/example-user"},
			targetType:        awslib.User{},
			targetName:        "example-user",
			targetAccountID:   "123456789012",
			targetPartitionID: "aws",
			targetString:      "arn:aws:iam::123456789012:user/example-user",
		},
		"RoleNameFromFlags": {
			flags:             configurators.BootstrapFlags{AttachToRole: "example-role"},
			accountID:         "123456789012",
			partitionID:       "aws",
			targetType:        awslib.Role{},
			targetName:        "example-role",
			targetAccountID:   "123456789012",
			targetPartitionID: "aws",
			targetString:      "arn:aws:iam::123456789012:role/example-role",
		},
		"RoleNameWithPathFromFlags": {
			flags:             configurators.BootstrapFlags{AttachToRole: "/some/path/example-role"},
			accountID:         "123456789012",
			partitionID:       "aws",
			targetType:        awslib.Role{},
			targetName:        "example-role",
			targetAccountID:   "123456789012",
			targetPartitionID: "aws",
			targetString:      "arn:aws:iam::123456789012:role/some/path/example-role",
		},
		"RoleARNFromFlags": {
			flags:             configurators.BootstrapFlags{AttachToRole: "arn:aws:iam::123456789012:role/example-role"},
			targetType:        awslib.Role{},
			targetName:        "example-role",
			targetAccountID:   "123456789012",
			targetPartitionID: "aws",
			targetString:      "arn:aws:iam::123456789012:role/example-role",
		},
		"UserFromIdentity": {
			flags:             configurators.BootstrapFlags{},
			identity:          userIdentity,
			targetType:        awslib.User{},
			targetName:        userIdentity.GetName(),
			targetAccountID:   userIdentity.GetAccountID(),
			targetPartitionID: userIdentity.GetPartition(),
			targetString:      "arn:aws:iam::123456789012:user/example-user",
		},
		"RoleFromIdentity": {
			flags:             configurators.BootstrapFlags{},
			identity:          roleIdentity,
			targetType:        awslib.Role{},
			targetName:        roleIdentity.GetName(),
			targetAccountID:   roleIdentity.GetAccountID(),
			targetPartitionID: roleIdentity.GetPartition(),
			targetString:      "arn:aws:iam::123456789012:role/example-role",
		},
		"DefaultTarget": {
			flags:             configurators.BootstrapFlags{},
			accountID:         "*",
			partitionID:       "*",
			targetType:        awslib.User{},
			targetName:        defaultAttachUser,
			targetAccountID:   "*",
			targetPartitionID: "*",
			targetString:      "arn:*:iam::*:user/username",
		},
		"AssumedRoleIdentity": {
			flags:             configurators.BootstrapFlags{},
			identity:          assumedRoleIdentity,
			targetType:        awslib.Role{},
			targetName:        assumedRoleIdentity.GetName(),
			targetAccountID:   assumedRoleIdentity.GetAccountID(),
			targetPartitionID: assumedRoleIdentity.GetPartition(),
			targetString:      "arn:aws:iam::123456789012:role/example-role",
			iamClient:         &iamMock{partition: "aws", account: "123456789012"},
		},
		"AssumedRoleIdentityForRoleWithPath": {
			flags:             configurators.BootstrapFlags{},
			identity:          assumedRoleIdentity,
			targetType:        awslib.Role{},
			targetName:        assumedRoleIdentity.GetName(),
			targetAccountID:   assumedRoleIdentity.GetAccountID(),
			targetPartitionID: assumedRoleIdentity.GetPartition(),
			targetString:      "arn:aws:iam::123456789012:role/some/path/example-role",
			iamClient:         &iamMock{partition: "aws", account: "123456789012", addPath: "/some/path/"},
		},
		"AssumedRoleIdentityWithoutIAMPermissions": {
			flags:             configurators.BootstrapFlags{},
			identity:          assumedRoleIdentity,
			targetType:        awslib.Role{},
			targetName:        assumedRoleIdentity.GetName(),
			targetAccountID:   assumedRoleIdentity.GetAccountID(),
			targetPartitionID: assumedRoleIdentity.GetPartition(),
			targetString:      "arn:aws:iam::123456789012:role/example-role",
			iamClient:         &iamMock{unauthorized: true},
			wantErrContains:   "Policies cannot be attached to an assumed-role",
		},
		"AssumedRoleIdentityWithRoleFromFlags": {
			flags:             configurators.BootstrapFlags{AttachToRole: "arn:aws:iam::123456789012:role/some/path/example-role"},
			identity:          assumedRoleIdentity,
			targetType:        awslib.Role{},
			targetName:        "example-role",
			targetAccountID:   "123456789012",
			targetPartitionID: "aws",
			targetString:      "arn:aws:iam::123456789012:role/some/path/example-role",
			iamClient:         &iamMock{unauthorized: true},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			target, err := policiesTarget(test.flags, test.accountID, test.partitionID, test.identity, test.iamClient)
			if test.wantErrContains != "" {
				require.ErrorContains(t, err, test.wantErrContains)
				return
			}
			require.NoError(t, err)
			require.IsType(t, test.targetType, target)
			require.Equal(t, test.targetName, target.GetName())
			require.Equal(t, test.targetAccountID, target.GetAccountID())
			require.Equal(t, test.targetPartitionID, target.GetPartition())
			require.Equal(t, test.targetString, target.String())
		})
	}
}

func TestAWSDocumentConfigurator(t *testing.T) {
	var err error
	ctx := context.Background()
	fileConfig := &config.FileConfig{
		Proxy: config.Proxy{
			PublicAddr: []string{"proxy.example.org:443"},
		},
		Discovery: config.Discovery{
			AWSMatchers: []config.AWSMatcher{
				{
					Types:   []string{"ec2"},
					Regions: []string{"eu-central-1"},
					SSM:     config.AWSSSM{DocumentName: "document"},
				},
			},
		},
	}
	require.NoError(t, fileConfig.CheckAndSetDefaults())
	serviceConfig := servicecfg.MakeDefaultConfig()
	require.NoError(t, config.ApplyFileConfig(fileConfig, serviceConfig))

	config := ConfiguratorConfig{
		AWSSession:   &awssession.Session{},
		AWSIAMClient: &iamMock{},
		AWSSTSClient: &STSMock{ARN: "arn:aws:iam::1234567:role/example-role"},
		AWSSSMClient: &SSMMock{
			t: t,
			expectedInput: &ssm.CreateDocumentInput{
				Content:        aws.String(EC2DiscoverySSMDocument("https://proxy.example.org:443")),
				DocumentType:   aws.String("Command"),
				DocumentFormat: aws.String("YAML"),
				Name:           aws.String("document"),
			},
		},
		ServiceConfig: serviceConfig,
		Flags: configurators.BootstrapFlags{
			Service:             configurators.DiscoveryService,
			ForceEC2Permissions: true,
		},
		Policies: &policiesMock{
			upsertArn: "policies-arn",
		},
	}
	configurator, err := NewAWSConfigurator(config)
	require.NoError(t, err)
	require.False(t, configurator.IsEmpty())

	// Execute actions.
	actionCtx := &configurators.ConfiguratorActionContext{}
	for _, action := range configurator.Actions() {
		err = action.Execute(ctx, actionCtx)
		require.NoError(t, err)
	}

}

// TestAWSConfigurator tests all actions together.
func TestAWSConfigurator(t *testing.T) {
	var err error
	ctx := context.Background()

	config := ConfiguratorConfig{
		AWSSession:    &awssession.Session{},
		AWSIAMClient:  &iamMock{},
		AWSSTSClient:  &STSMock{ARN: "arn:aws:iam::1234567:role/example-role"},
		AWSSSMClient:  &SSMMock{},
		ServiceConfig: &servicecfg.Config{},
		Flags: configurators.BootstrapFlags{
			AttachToUser:        "some-user",
			ForceRDSPermissions: true,
		},
		Policies: &policiesMock{
			upsertArn: "policies-arn",
		},
	}

	configurator, err := NewAWSConfigurator(config)
	require.NoError(t, err)
	require.False(t, configurator.IsEmpty())

	// Execute actions.
	actionCtx := &configurators.ConfiguratorActionContext{}
	for _, action := range configurator.Actions() {
		err = action.Execute(ctx, actionCtx)
		require.NoError(t, err)
	}

	config.Flags.Service = configurators.DiscoveryService
	config.Flags.ForceEC2Permissions = true
	config.Flags.Proxy = "proxy.xyz"

	configurator, err = NewAWSConfigurator(config)
	require.NoError(t, err)
	require.False(t, configurator.IsEmpty())

	// Execute actions.
	actionCtx = &configurators.ConfiguratorActionContext{}
	for _, action := range configurator.Actions() {
		err = action.Execute(ctx, actionCtx)
		require.NoError(t, err)
	}

}

func TestExtractTargetConfig(t *testing.T) {
	t.Parallel()
	role1 := "arn:aws:iam::123456789012:role/role-1"
	role2 := "arn:aws:iam::123456789012:role/role-2"
	role3 := "arn:aws:iam::123456789012:role/role-3"
	role4 := "arn:aws:iam::123456789012:role/role-4"
	role5 := "arn:aws:iam::123456789012:role/role-5"
	role6 := "arn:aws:iam::123456789012:role/role-6"
	role7 := "arn:aws:iam::123456789012:role/role-7"
	roleTarget, err := awslib.IdentityFromArn("arn:aws:iam::123456789012:role/target-role")
	require.NoError(t, err)
	manualModeRoleTarget, err := awslib.IdentityFromArn(
		buildIAMARN(
			targetIdentityARNSectionPlaceholder,
			targetIdentityARNSectionPlaceholder,
			"role", "target-role",
		))
	require.NoError(t, err)

	tests := map[string]struct {
		target  awslib.Identity
		flags   configurators.BootstrapFlags
		cfg     *servicecfg.Config
		want    targetConfig
		wantErr string
	}{
		"target in assume roles": {
			target: roleTarget,
			flags:  configurators.BootstrapFlags{ForceAssumesRoles: role1},
			cfg: &servicecfg.Config{
				// check discovery resources are not included
				Discovery: servicecfg.DiscoveryConfig{
					AWSMatchers: []types.AWSMatcher{
						{Types: []string{services.AWSMatcherRDSProxy}, AssumeRole: &types.AssumeRole{RoleARN: roleTarget.String()}},
					},
				},
				Databases: servicecfg.DatabasesConfig{
					Databases: []servicecfg.Database{
						{Name: "db1", AWS: servicecfg.DatabaseAWS{AssumeRoleARN: role2}},
						{Name: "db2", AWS: servicecfg.DatabaseAWS{AssumeRoleARN: role3, ExternalID: "foo"}},
						{Name: "db3"},
						{Name: "db4", AWS: servicecfg.DatabaseAWS{AssumeRoleARN: roleTarget.String()}},
					},
					AWSMatchers: []types.AWSMatcher{
						{Types: []string{services.AWSMatcherRDS, services.AWSMatcherEC2}, AssumeRole: &types.AssumeRole{RoleARN: role4}},
						{Types: []string{services.AWSMatcherRDS, services.AWSMatcherEC2}, AssumeRole: &types.AssumeRole{RoleARN: role5, ExternalID: "foo"}},
						{Types: []string{services.AWSMatcherEC2}, AssumeRole: &types.AssumeRole{RoleARN: role6}},
						{Types: []string{services.AWSMatcherElastiCache}},
						{Types: []string{services.AWSMatcherRedshift}, AssumeRole: &types.AssumeRole{RoleARN: roleTarget.String()}},
					},
				},
			},
			want: targetConfig{
				identity:        roleTarget,
				assumesAWSRoles: []string{role1},
				databases:       []*servicecfg.Database{{Name: "db4", AWS: servicecfg.DatabaseAWS{AssumeRoleARN: roleTarget.String()}}},
				awsMatchers: []types.AWSMatcher{
					{Types: []string{services.AWSMatcherRedshift}, AssumeRole: &types.AssumeRole{RoleARN: roleTarget.String()}},
				},
			},
		},
		"target in discovery assume roles": {
			target: roleTarget,
			flags:  configurators.BootstrapFlags{ForceAssumesRoles: role1, Service: configurators.DiscoveryService},
			cfg: &servicecfg.Config{
				Discovery: servicecfg.DiscoveryConfig{
					AWSMatchers: []types.AWSMatcher{
						{Types: []string{services.AWSMatcherRDSProxy}, AssumeRole: &types.AssumeRole{RoleARN: roleTarget.String()}},
					},
				},
				// check that database service resources are not included.
				Databases: servicecfg.DatabasesConfig{
					Databases: []servicecfg.Database{
						{Name: "db1", AWS: servicecfg.DatabaseAWS{AssumeRoleARN: roleTarget.String()}},
					},
					AWSMatchers: []types.AWSMatcher{
						{Types: []string{services.AWSMatcherRedshift}, AssumeRole: &types.AssumeRole{RoleARN: roleTarget.String()}},
					},
				},
			},
			want: targetConfig{
				identity:        roleTarget,
				assumesAWSRoles: []string{role1},
				awsMatchers: []types.AWSMatcher{
					{Types: []string{services.AWSMatcherRDSProxy}, AssumeRole: &types.AssumeRole{RoleARN: roleTarget.String()}},
				},
			},
		},
		"target in discovery assume roles (boostrapping database service)": {
			target: roleTarget,
			flags:  configurators.BootstrapFlags{ForceAssumesRoles: role1, Service: configurators.DatabaseServiceByDiscoveryServiceConfig},
			cfg: &servicecfg.Config{
				Discovery: servicecfg.DiscoveryConfig{
					AWSMatchers: []types.AWSMatcher{
						{Types: []string{services.AWSMatcherRDSProxy}, AssumeRole: &types.AssumeRole{RoleARN: roleTarget.String()}},
					},
				},
				// check that database service resources are not included.
				Databases: servicecfg.DatabasesConfig{
					Databases: []servicecfg.Database{
						{Name: "db1", AWS: servicecfg.DatabaseAWS{AssumeRoleARN: roleTarget.String()}},
					},
					AWSMatchers: []types.AWSMatcher{
						{Types: []string{services.AWSMatcherRedshift}, AssumeRole: &types.AssumeRole{RoleARN: roleTarget.String()}},
					},
				},
			},
			want: targetConfig{
				identity:        roleTarget,
				assumesAWSRoles: []string{role1},
				awsMatchers: []types.AWSMatcher{
					{Types: []string{services.AWSMatcherRDSProxy}, AssumeRole: &types.AssumeRole{RoleARN: roleTarget.String()}},
				},
			},
		},
		"target not in assume roles": {
			target: roleTarget,
			flags:  configurators.BootstrapFlags{ForceAssumesRoles: role1},
			cfg: &servicecfg.Config{
				// check that discovery service resources are not included.
				Discovery: servicecfg.DiscoveryConfig{
					AWSMatchers: []types.AWSMatcher{
						{Types: []string{services.AWSMatcherEC2}},
					},
				},
				Databases: servicecfg.DatabasesConfig{
					Databases: []servicecfg.Database{
						{Name: "db1", AWS: servicecfg.DatabaseAWS{AssumeRoleARN: role2}},
						{Name: "db2", AWS: servicecfg.DatabaseAWS{AssumeRoleARN: role3, ExternalID: "foo"}},
						{Name: "db3"},
					},
					AWSMatchers: []types.AWSMatcher{
						// rds/ec2 matcher's assume role should be added because rds assume role is supported.
						{Types: []string{services.AWSMatcherRDS, services.AWSMatcherEC2}, AssumeRole: &types.AssumeRole{RoleARN: role4}},
						// ec2-only matcher's assume role should not be added because it's not supported.
						{Types: []string{services.AWSMatcherEC2}, AssumeRole: &types.AssumeRole{RoleARN: role5}},
						{Types: []string{services.AWSMatcherRDS, services.AWSMatcherEC2}, AssumeRole: &types.AssumeRole{RoleARN: role6, ExternalID: "foo"}},
						// matcher without assume role should be added to matchers
						{Types: []string{services.AWSMatcherElastiCache}},
					},
					ResourceMatchers: []services.ResourceMatcher{
						// dynamic resources' assume role should be added.
						{Labels: types.Labels{"env": []string{"dev"}}, AWS: services.ResourceMatcherAWS{AssumeRoleARN: role7}},
					},
				},
			},
			want: targetConfig{
				identity:        roleTarget,
				assumesAWSRoles: []string{role1, role2, role3, role4, role6, role7},
				databases:       []*servicecfg.Database{{Name: "db3"}},
				awsMatchers: []types.AWSMatcher{
					{Types: []string{services.AWSMatcherElastiCache}},
				},
			},
		},
		"target not in discovery roles": {
			target: roleTarget,
			flags:  configurators.BootstrapFlags{ForceAssumesRoles: role1, Service: configurators.DiscoveryService},
			cfg: &servicecfg.Config{
				Discovery: servicecfg.DiscoveryConfig{
					AWSMatchers: []types.AWSMatcher{
						{Types: []string{services.AWSMatcherRDSProxy}, AssumeRole: &types.AssumeRole{RoleARN: role2}},
						{Types: []string{services.AWSMatcherRDSProxy}, AssumeRole: &types.AssumeRole{RoleARN: role3, ExternalID: "foo"}},
						{Types: []string{services.AWSMatcherEC2}},
					},
				},
				// check that database service resources are not included.
				Databases: servicecfg.DatabasesConfig{
					Databases: []servicecfg.Database{
						{Name: "db3"},
					},
					AWSMatchers: []types.AWSMatcher{
						{Types: []string{services.AWSMatcherElastiCache}, AssumeRole: &types.AssumeRole{RoleARN: role4}},
					},
				},
			},
			want: targetConfig{
				identity:        roleTarget,
				assumesAWSRoles: []string{role1, role2, role3},
				awsMatchers: []types.AWSMatcher{
					{Types: []string{services.AWSMatcherEC2}},
				},
			},
		},
		"manual mode role name target assuming config roles is an error": {
			target: manualModeRoleTarget,
			flags:  configurators.BootstrapFlags{ForceAssumesRoles: role1, Manual: true},
			cfg: &servicecfg.Config{
				Discovery: servicecfg.DiscoveryConfig{
					AWSMatchers: []types.AWSMatcher{
						{Types: []string{services.AWSMatcherRDSProxy}, AssumeRole: &types.AssumeRole{RoleARN: roleTarget.String()}},
					},
				},
				Databases: servicecfg.DatabasesConfig{
					Databases: []servicecfg.Database{
						{Name: "db1", AWS: servicecfg.DatabaseAWS{AssumeRoleARN: role2}},
					},
					AWSMatchers: []types.AWSMatcher{
						{Types: []string{services.AWSMatcherRDS, services.AWSMatcherEC2}, AssumeRole: &types.AssumeRole{RoleARN: role4}},
					},
				},
			},
			wantErr: "please specify the full role ARN",
		},
		"manual mode role name target assuming only forced roles is ok": {
			target: manualModeRoleTarget,
			flags:  configurators.BootstrapFlags{ForceAssumesRoles: role1, Manual: true},
			cfg: &servicecfg.Config{
				Databases: servicecfg.DatabasesConfig{
					AWSMatchers: []types.AWSMatcher{
						{Types: []string{services.AWSMatcherRDS, services.AWSMatcherEC2}},
					},
				},
			},
			want: targetConfig{
				identity:        roleTarget,
				assumesAWSRoles: []string{role1},
				awsMatchers: []types.AWSMatcher{
					{Types: []string{services.AWSMatcherRDS, services.AWSMatcherEC2}},
				},
			},
		},
		"manual mode role name target without any assume roles is ok": {
			target: manualModeRoleTarget,
			flags:  configurators.BootstrapFlags{Manual: true},
			cfg: &servicecfg.Config{
				Databases: servicecfg.DatabasesConfig{
					AWSMatchers: []types.AWSMatcher{
						{Types: []string{services.AWSMatcherRDS, services.AWSMatcherEC2}},
					},
				},
			},
			want: targetConfig{
				identity: roleTarget,
				awsMatchers: []types.AWSMatcher{
					{Types: []string{services.AWSMatcherRDS, services.AWSMatcherEC2}},
				},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := getTargetConfig(tt.flags, tt.cfg, tt.target)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.target, got.identity)
			// for test convenience, use cmp.Diff to equate []Type(nil) and []Type{}.
			diff := cmp.Diff(tt.want, got,
				cmpopts.EquateEmpty(),
				cmp.AllowUnexported(targetConfig{}),
				// since diff is allowing unexported types and identity can
				// contain the external type arn.ARN, ignore it in the diff.
				// check it separately with require.Equal above.
				cmpopts.IgnoreFields(targetConfig{}, "identity"))
			require.Empty(t, diff)
		})
	}
}

func TestIsTargetAssumeRole(t *testing.T) {
	t.Parallel()
	userTarget, err := awslib.IdentityFromArn("arn:aws:iam::123456789012:user/example-user")
	require.NoError(t, err)
	role1 := "arn:aws:iam::123456789012:role/role-1"
	roleTarget, err := awslib.IdentityFromArn("arn:aws:iam::123456789012:role/target-role")
	require.NoError(t, err)

	tests := map[string]struct {
		target    awslib.Identity
		flags     configurators.BootstrapFlags
		matchers  []types.AWSMatcher
		databases []*servicecfg.Database
		resources []services.ResourceMatcher
		want      bool
	}{
		"target in matchers": {
			target: roleTarget,
			matchers: []types.AWSMatcher{{
				Types:      []string{services.AWSMatcherRDS},
				Regions:    []string{"us-west-1"},
				AssumeRole: &types.AssumeRole{RoleARN: roleTarget.String()},
			}},
			want: true,
		},
		"target in databases": {
			target: roleTarget,
			databases: []*servicecfg.Database{{
				AWS: servicecfg.DatabaseAWS{
					AssumeRoleARN: roleTarget.String(),
				},
			}},
			want: true,
		},
		"target in resources": {
			target: roleTarget,
			resources: []services.ResourceMatcher{
				{
					Labels: types.Labels{
						"env": []string{"prod"},
					},
				},
				{
					Labels: types.Labels{
						"env": []string{"dev"},
					},
					AWS: services.ResourceMatcherAWS{
						AssumeRoleARN: roleTarget.String(),
						ExternalID:    "external-id",
					},
				},
			},
			want: true,
		},
		"target is not a role": {
			target: userTarget,
			want:   false,
		},
		"target not in anything": {
			target: roleTarget,
			flags:  configurators.BootstrapFlags{ForceAssumesRoles: role1},
			matchers: []types.AWSMatcher{{
				Types:      []string{services.AWSMatcherRDS},
				Regions:    []string{"us-west-1"},
				AssumeRole: &types.AssumeRole{RoleARN: role1},
			}},
			databases: []*servicecfg.Database{{
				AWS: servicecfg.DatabaseAWS{
					AssumeRoleARN: role1,
				},
			}},
			want: false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := isTargetAWSAssumeRole(tt.flags, tt.matchers, tt.databases, tt.resources, tt.target)
			require.Equal(t, tt.want, got)
		})
	}
}

type policiesMock struct {
	awslib.Policies

	upsertArn           string
	upsertError         error
	attachError         error
	attachBoundaryError error
}

func (p *policiesMock) Upsert(context.Context, *awslib.Policy) (string, error) {
	return p.upsertArn, p.upsertError
}

func (p *policiesMock) Attach(context.Context, string, awslib.Identity) error {
	return p.attachError
}

func (p *policiesMock) AttachBoundary(context.Context, string, awslib.Identity) error {
	return p.attachBoundaryError
}

type STSMock struct {
	stsiface.STSAPI
	ARN               string
	callerIdentityErr error
}

func (m *STSMock) GetCallerIdentityWithContext(aws.Context, *sts.GetCallerIdentityInput, ...request.Option) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{
		Arn: aws.String(m.ARN),
	}, m.callerIdentityErr
}

type SSMMock struct {
	ssmiface.SSMAPI

	t             *testing.T
	expectedInput *ssm.CreateDocumentInput
}

func (m *SSMMock) CreateDocumentWithContext(ctx aws.Context, input *ssm.CreateDocumentInput, opts ...request.Option) (*ssm.CreateDocumentOutput, error) {

	m.t.Helper()
	require.Equal(m.t, m.expectedInput, input)

	return nil, nil
}

type iamMock struct {
	iamiface.IAMAPI
	unauthorized bool
	partition    string
	account      string
	addPath      string
}

func (m *iamMock) GetRole(input *iam.GetRoleInput) (*iam.GetRoleOutput, error) {
	if m.unauthorized {
		return nil, trace.AccessDenied("unauthorized")
	}
	roleName := aws.StringValue(input.RoleName)
	path := m.addPath
	if path == "" {
		path = "/"
	}
	arn := fmt.Sprintf("arn:%s:iam::%s:role%s%s", m.partition, m.account, path, roleName)
	return &iam.GetRoleOutput{Role: &iam.Role{Arn: &arn}}, nil
}
