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

package databases

import (
	"context"
	"sort"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/trace"
)

func TestAWSIAMDocuments(t *testing.T) {
	userTarget, err := awslib.IdentityFromArn("arn:aws:iam::1234567:user/example-user")
	require.NoError(t, err)

	roleTarget, err := awslib.IdentityFromArn("arn:aws:iam::1234567:role/example-role")
	require.NoError(t, err)

	unknownIdentity, err := awslib.IdentityFromArn("arn:aws:iam::1234567:ec2/example-ec2")
	require.NoError(t, err)

	tests := map[string]struct {
		returnError        bool
		flags              BootstrapFlags
		fileConfig         *config.FileConfig
		target             awslib.Identity
		statements         []*awslib.Statement
		boundaryStatements []*awslib.Statement
	}{
		"RDSAutoDiscoveryToUser": {
			target: userTarget,
			fileConfig: &config.FileConfig{
				Databases: config.Databases{
					AWSMatchers: []config.AWSMatcher{
						{Types: []string{types.DatabaseTypeRDS}, Regions: []string{"us-west-2"}},
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
					AWSMatchers: []config.AWSMatcher{
						{Types: []string{types.DatabaseTypeRDS}, Regions: []string{"us-west-2"}},
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
					AWSMatchers: []config.AWSMatcher{
						{Types: []string{types.DatabaseTypeRedshift}, Regions: []string{"us-west-2"}},
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
					AWSMatchers: []config.AWSMatcher{
						{Types: []string{types.DatabaseTypeRedshift}, Regions: []string{"us-west-2"}},
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
					Databases: []*config.Database{
						{
							Name: "redshift-cluster-1",
							URI:  "redshift-cluster-1.abcdefghijkl.us-west-2.redshift.amazonaws.com:5439",
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
		"AutoDiscoveryUnknownIdentity": {
			returnError: true,
			target:      unknownIdentity,
			fileConfig: &config.FileConfig{
				Databases: config.Databases{
					AWSMatchers: []config.AWSMatcher{
						{Types: []string{types.DatabaseTypeRDS}, Regions: []string{"us-west-2"}},
					},
				},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			policy, policyErr := buildPolicyDocument(test.flags, test.fileConfig, test.target)
			boundary, boundaryErr := buildPolicyBoundaryDocument(test.flags, test.fileConfig, test.target)

			if test.returnError {
				require.Error(t, policyErr)
				require.Error(t, boundaryErr)
				return
			}

			sortStringsTrans := cmp.Transformer("SortStrings", func(in []string) []string {
				out := append([]string(nil), in...) // Copy input to avoid mutating it
				sort.Strings(out)
				return out
			})
			require.Empty(t, cmp.Diff(test.statements, policy.Document.Statements, sortStringsTrans))
			require.Empty(t, cmp.Diff(test.boundaryStatements, boundary.Document.Statements, sortStringsTrans))
		})
	}
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

			actionCtx := &ConfiguratorActionContext{}
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
		actionCtx   *ConfiguratorActionContext
	}{
		"AttachPoliciesToUser": {
			target:   userTarget,
			policies: &policiesMock{},
			actionCtx: &ConfiguratorActionContext{
				AWSPolicyArn:         "policy-arn",
				AWSPolicyBoundaryArn: "policy-boundary-arn",
			},
		},
		"AttachPoliciesToRole": {
			target:   roleTarget,
			policies: &policiesMock{},
			actionCtx: &ConfiguratorActionContext{
				AWSPolicyArn:         "policy-arn",
				AWSPolicyBoundaryArn: "policy-boundary-arn",
			},
		},
		"MissingPolicyArn": {
			returnError: true,
			target:      roleTarget,
			policies:    &policiesMock{},
			actionCtx: &ConfiguratorActionContext{
				AWSPolicyBoundaryArn: "policy-boundary-arn",
			},
		},
		"MissingPolicyBoundaryArn": {
			returnError: true,
			target:      roleTarget,
			policies:    &policiesMock{},
			actionCtx: &ConfiguratorActionContext{
				AWSPolicyArn: "policy-arn",
			},
		},
		"AttachPolicyFailure": {
			returnError: true,
			target:      roleTarget,
			policies: &policiesMock{
				attachError: trace.NotImplemented("attach policy not implemented"),
			},
			actionCtx: &ConfiguratorActionContext{
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
			actionCtx: &ConfiguratorActionContext{
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
	userIdentity, err := awslib.IdentityFromArn("arn:aws:iam::1234567:user/example-user")
	require.NoError(t, err)

	roleIdentity, err := awslib.IdentityFromArn("arn:aws:iam::1234567:role/example-role")
	require.NoError(t, err)

	tests := map[string]struct {
		flags           BootstrapFlags
		identity        awslib.Identity
		accountID       string
		targetType      awslib.Identity
		targetName      string
		targetAccountID string
	}{
		"UserNameFromFlags": {
			flags:           BootstrapFlags{AttachToUser: "example-user"},
			accountID:       "123456",
			targetType:      awslib.User{},
			targetName:      "example-user",
			targetAccountID: "123456",
		},
		"UserARNFromFlags": {
			flags:           BootstrapFlags{AttachToUser: "arn:aws:iam::123456:user/example-user"},
			targetType:      awslib.User{},
			targetName:      "example-user",
			targetAccountID: "123456",
		},
		"RoleNameFromFlags": {
			flags:           BootstrapFlags{AttachToRole: "example-role"},
			accountID:       "123456",
			targetType:      awslib.Role{},
			targetName:      "example-role",
			targetAccountID: "123456",
		},
		"RoleARNFromFlags": {
			flags:           BootstrapFlags{AttachToRole: "arn:aws:iam::123456:role/example-role"},
			targetType:      awslib.Role{},
			targetName:      "example-role",
			targetAccountID: "123456",
		},
		"UserFromIdentity": {
			flags:           BootstrapFlags{},
			identity:        userIdentity,
			targetType:      awslib.User{},
			targetName:      userIdentity.GetName(),
			targetAccountID: userIdentity.GetAccountID(),
		},
		"RoleFromIdentity": {
			flags:           BootstrapFlags{},
			identity:        roleIdentity,
			targetType:      awslib.Role{},
			targetName:      roleIdentity.GetName(),
			targetAccountID: roleIdentity.GetAccountID(),
		},
		"DefaultTarget": {
			flags:           BootstrapFlags{},
			accountID:       "*",
			targetType:      awslib.User{},
			targetName:      defaultAttachUser,
			targetAccountID: "*",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			target, err := policiesTarget(test.flags, test.accountID, test.identity)
			require.NoError(t, err)
			require.IsType(t, test.targetType, target)
			require.Equal(t, test.targetName, target.GetName())
			require.Equal(t, test.targetAccountID, target.GetAccountID())
		})
	}
}

// TestAWSConfigurator tests all actions together.
func TestAWSConfigurator(t *testing.T) {
	var err error
	ctx := context.Background()

	config := AWSConfiguratorConfig{
		AWSSession:   &awssession.Session{},
		AWSSTSClient: &STSMock{ARN: "arn:aws:iam::1234567:role/example-role"},
		FileConfig:   &config.FileConfig{},
		Flags: BootstrapFlags{
			AttachToUser:        "some-user",
			ForceRDSPermissions: true,
		},
		Policies: &policiesMock{
			upsertArn: "polcies-arn",
		},
	}

	configurator, err := NewAWSConfigurator(config)
	require.NoError(t, err)
	require.False(t, configurator.IsEmpty())

	// Execute actions.
	actionCtx := &ConfiguratorActionContext{}
	for _, action := range configurator.Actions() {
		err = action.Execute(ctx, actionCtx)
		require.NoError(t, err)
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
