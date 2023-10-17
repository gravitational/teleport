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
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamTypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/config"
)

// TestConfigureExternalCloudAudit tests that ConfigureExternalCloudAudit
// creates a well-formatted IAM policy and attaches it to the correct role, and
// behaves well in error cases.
func TestConfigureExternalCloudAudit(t *testing.T) {
	ctx := context.Background()

	for _, tc := range []struct {
		desc                 string
		params               *config.IntegrationConfExternalCloudAuditIAM
		stsAccount           string
		existingRolePolicies map[string]map[string]string
		expectedRolePolicies map[string]map[string]string
		errorContains        []string
	}{
		{
			// A passing case with the account from sts:GetCallerIdentity
			desc: "passing",
			params: &config.IntegrationConfExternalCloudAuditIAM{
				Partition:            "aws",
				Region:               "us-west-2",
				Role:                 "test-role",
				Policy:               "test-policy",
				AuditEventsURI:       "s3://testbucket_noprefix",
				SessionRecordingsURI: "s3://testbucket/prefix",
				AthenaResultsURI:     "s3://transientbucket/results",
				AthenaWorkgroup:      "testworkgroup",
				GlueDatabase:         "testdb",
				GlueTable:            "testtable",
			},
			stsAccount: "12345678",
			existingRolePolicies: map[string]map[string]string{
				"test-role": {},
			},
			expectedRolePolicies: map[string]map[string]string{
				"test-role": {
					"test-policy": `{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "s3:PutObject",
                "s3:GetObject",
                "s3:GetObjectVersion",
                "s3:ListMultipartUploadParts",
                "s3:AbortMultipartUpload"
            ],
            "Resource": [
                "arn:aws:s3:::testbucket_noprefix/*",
                "arn:aws:s3:::testbucket/prefix/*",
                "arn:aws:s3:::transientbucket/results/*"
            ],
            "Sid": "ReadWriteSessionsAndEvents"
        },
        {
            "Effect": "Allow",
            "Action": [
                "athena:StartQueryExecution",
                "athena:GetQueryResults",
                "athena:GetQueryExecution"
            ],
            "Resource": "arn:aws:athena:us-west-2:12345678:workgroup/testworkgroup",
            "Sid": "AllowAthenaQuery"
        },
        {
            "Effect": "Allow",
            "Action": [
                "glue:GetTable",
                "glue:GetTableVersion",
                "glue:GetTableVersions",
                "glue:UpdateTable"
            ],
            "Resource": [
                "arn:aws:glue:us-west-2:12345678:catalog",
                "arn:aws:glue:us-west-2:12345678:database/testdb",
                "arn:aws:glue:us-west-2:12345678:table/testdb/testtable"
            ],
            "Sid": "FullAccessOnGlueTable"
        }
    ]
}`,
				},
			},
		},
		{
			desc: "alternate partition and region",
			params: &config.IntegrationConfExternalCloudAuditIAM{
				Partition:            "aws-cn",
				Region:               "cn-north-1",
				Role:                 "test-role",
				Policy:               "test-policy",
				AuditEventsURI:       "s3://testbucket_noprefix",
				SessionRecordingsURI: "s3://testbucket/prefix",
				AthenaResultsURI:     "s3://transientbucket/results",
				AthenaWorkgroup:      "testworkgroup",
				GlueDatabase:         "testdb",
				GlueTable:            "testtable",
			},
			stsAccount: "12345678",
			existingRolePolicies: map[string]map[string]string{
				"test-role": {},
			},
			expectedRolePolicies: map[string]map[string]string{
				"test-role": {
					"test-policy": `{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "s3:PutObject",
                "s3:GetObject",
                "s3:GetObjectVersion",
                "s3:ListMultipartUploadParts",
                "s3:AbortMultipartUpload"
            ],
            "Resource": [
                "arn:aws-cn:s3:::testbucket_noprefix/*",
                "arn:aws-cn:s3:::testbucket/prefix/*",
                "arn:aws-cn:s3:::transientbucket/results/*"
            ],
            "Sid": "ReadWriteSessionsAndEvents"
        },
        {
            "Effect": "Allow",
            "Action": [
                "athena:StartQueryExecution",
                "athena:GetQueryResults",
                "athena:GetQueryExecution"
            ],
            "Resource": "arn:aws-cn:athena:cn-north-1:12345678:workgroup/testworkgroup",
            "Sid": "AllowAthenaQuery"
        },
        {
            "Effect": "Allow",
            "Action": [
                "glue:GetTable",
                "glue:GetTableVersion",
                "glue:GetTableVersions",
                "glue:UpdateTable"
            ],
            "Resource": [
                "arn:aws-cn:glue:cn-north-1:12345678:catalog",
                "arn:aws-cn:glue:cn-north-1:12345678:database/testdb",
                "arn:aws-cn:glue:cn-north-1:12345678:table/testdb/testtable"
            ],
            "Sid": "FullAccessOnGlueTable"
        }
    ]
}`,
				},
			},
		},
		{
			desc: "bad uri",
			params: &config.IntegrationConfExternalCloudAuditIAM{
				Partition:            "aws",
				Region:               "us-west-2",
				Role:                 "test-role",
				SessionRecordingsURI: "file:///tmp/recordings",
				AuditEventsURI:       "s3://longtermbucket/events",
				AthenaResultsURI:     "s3://transientbucket/results",
				AthenaWorkgroup:      "testworkgroup",
				GlueDatabase:         "testdb",
				GlueTable:            "testtable",
			},
			stsAccount: "12345678",
			existingRolePolicies: map[string]map[string]string{
				"test-role": {},
			},
			errorContains: []string{
				"parsing session recordings URI",
				"URI scheme must be s3",
			},
		},
		{
			desc: "role not found",
			params: &config.IntegrationConfExternalCloudAuditIAM{
				Partition:            "aws",
				Region:               "us-west-2",
				Role:                 "bad-role",
				SessionRecordingsURI: "s3://longtermbucket/recordings",
				AuditEventsURI:       "s3://longtermbucket/events",
				AthenaResultsURI:     "s3://transientbucket/results",
				AthenaWorkgroup:      "testworkgroup",
				GlueDatabase:         "testdb",
				GlueTable:            "testtable",
			},
			stsAccount: "12345678",
			errorContains: []string{
				`role "bad-role" not found`,
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			currentRolePolicies := cloneRolePolicies(tc.existingRolePolicies)
			clt := &fakeConfigureExternalCloudAuditClient{
				account:      tc.stsAccount,
				rolePolicies: currentRolePolicies,
			}
			err := ConfigureExternalCloudAudit(ctx, clt, tc.params)
			if len(tc.errorContains) > 0 {
				for _, msg := range tc.errorContains {
					require.ErrorContains(t, err, msg)
				}
				return
			}
			require.NoError(t, err, trace.DebugReport(err))
			require.Equal(t, tc.expectedRolePolicies, currentRolePolicies)
		})
	}
}

type fakeConfigureExternalCloudAuditClient struct {
	account string
	// rolePolicies is a nested map holding the state of existing roles and
	// their attached policies. Each outer key is a role name, the value is a
	// map of policy names to policy documents.
	rolePolicies map[string]map[string]string
}

func (f *fakeConfigureExternalCloudAuditClient) PutRolePolicy(ctx context.Context, input *iam.PutRolePolicyInput, opts ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error) {
	roleName := aws.ToString(input.RoleName)
	if _, roleExists := f.rolePolicies[roleName]; !roleExists {
		return nil, &iamTypes.NoSuchEntityException{
			Message: aws.String(fmt.Sprintf("role %q does not exist", roleName)),
		}
	}
	if f.rolePolicies[roleName] == nil {
		f.rolePolicies[roleName] = make(map[string]string)
	}
	f.rolePolicies[roleName][aws.ToString(input.PolicyName)] = aws.ToString(input.PolicyDocument)
	return &iam.PutRolePolicyOutput{}, nil
}

func (f *fakeConfigureExternalCloudAuditClient) GetCallerIdentity(ctx context.Context, input *sts.GetCallerIdentityInput, opts ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{
		Account: aws.String(f.account),
		Arn:     aws.String("some_ignored_arn"),
		UserId:  aws.String("some_ignored_user_id"),
	}, nil
}

func cloneRolePolicies(in map[string]map[string]string) map[string]map[string]string {
	out := make(map[string]map[string]string, len(in))
	for role, policies := range in {
		out[role] = make(map[string]string, len(policies))
		for policyName, policyDoc := range policies {
			out[role][policyName] = policyDoc
		}
	}
	return out
}
