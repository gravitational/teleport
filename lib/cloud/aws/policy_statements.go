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

import "fmt"

var (
	allResources = []string{"*"}
)

// StatementForIAMEditRolePolicy returns a IAM Policy Statement which allows editting Role Policy
// of the resources.
func StatementForIAMEditRolePolicy(resources ...string) *Statement {
	return &Statement{
		Effect:    EffectAllow,
		Actions:   []string{"iam:GetRolePolicy", "iam:PutRolePolicy", "iam:DeleteRolePolicy"},
		Resources: resources,
	}
}

// StatementForIAMEditUserPolicy returns a IAM Policy Statement which allows editting User Policy
// of the resources.
func StatementForIAMEditUserPolicy(resources ...string) *Statement {
	return &Statement{
		Effect:    EffectAllow,
		Actions:   []string{"iam:GetUserPolicy", "iam:PutUserPolicy", "iam:DeleteUserPolicy"},
		Resources: resources,
	}
}

// StatementForECSManageService returns the statement that allows managing the ECS Service deployed
// by DeployService (AWS OIDC Integration).
func StatementForECSManageService() *Statement {
	return &Statement{
		Effect: EffectAllow,
		Actions: []string{
			"ecs:DescribeClusters", "ecs:CreateCluster", "ecs:PutClusterCapacityProviders",
			"ecs:DescribeServices", "ecs:CreateService", "ecs:UpdateService",
			"ecs:RegisterTaskDefinition", "ecs:DescribeTaskDefinition", "ecs:DeregisterTaskDefinition",

			// EC2 DescribeSecurityGroups is required so that the user can list the SG and then pick which ones they want to apply to the ECS Service.
			"ec2:DescribeSecurityGroups",
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
		Conditions: map[string]map[string]SliceOrString{
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
		},
		Resources: allResources,
	}
}
