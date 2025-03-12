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

package awsoidc

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/integrations/externalauditstorage/easconfig"
)

// TestConfigureExternalAuditStorage tests that ConfigureExternalAuditStorage
// creates a well-formatted IAM policy and attaches it to the correct role, and
// behaves well in error cases.
func TestConfigureExternalAuditStorage(t *testing.T) {
	ctx := context.Background()

	for _, tc := range []struct {
		desc                 string
		params               easconfig.ExternalAuditStorageConfiguration
		stsAccount           string
		existingRolePolicies map[string]map[string]string
		expectedRolePolicies map[string]map[string]string
		errorContains        []string
	}{
		{
			// A passing case with the account from sts:GetCallerIdentity
			desc: "passing",
			params: easconfig.ExternalAuditStorageConfiguration{
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
                "s3:AbortMultipartUpload",
                "s3:ListBucket",
                "s3:ListBucketVersions",
                "s3:ListBucketMultipartUploads",
                "s3:GetBucketOwnershipControls",
                "s3:GetBucketPublicAccessBlock",
                "s3:GetBucketObjectLockConfiguration",
                "s3:GetBucketVersioning",
                "s3:GetBucketLocation"
            ],
            "Resource": [
                "arn:aws:s3:::testbucket_noprefix",
                "arn:aws:s3:::testbucket_noprefix/*",
                "arn:aws:s3:::testbucket",
                "arn:aws:s3:::testbucket/prefix/*",
                "arn:aws:s3:::transientbucket",
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
			params: easconfig.ExternalAuditStorageConfiguration{
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
                "s3:AbortMultipartUpload",
                "s3:ListBucket",
                "s3:ListBucketVersions",
                "s3:ListBucketMultipartUploads",
                "s3:GetBucketOwnershipControls",
                "s3:GetBucketPublicAccessBlock",
                "s3:GetBucketObjectLockConfiguration",
                "s3:GetBucketVersioning",
                "s3:GetBucketLocation"
            ],
            "Resource": [
                "arn:aws-cn:s3:::testbucket_noprefix",
                "arn:aws-cn:s3:::testbucket_noprefix/*",
                "arn:aws-cn:s3:::testbucket",
                "arn:aws-cn:s3:::testbucket/prefix/*",
                "arn:aws-cn:s3:::transientbucket",
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
			params: easconfig.ExternalAuditStorageConfiguration{
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
			params: easconfig.ExternalAuditStorageConfiguration{
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
			clt := &fakeConfigureExternalAuditStorageClient{
				account:      tc.stsAccount,
				rolePolicies: currentRolePolicies,
			}
			err := ConfigureExternalAuditStorage(ctx, clt, tc.params)
			if len(tc.errorContains) > 0 {
				for _, msg := range tc.errorContains {
					require.ErrorContains(t, err, msg)
				}
				return
			}
			require.NoError(t, err, trace.DebugReport(err))
			require.Equal(t, tc.expectedRolePolicies, currentRolePolicies, cmp.Diff(tc.expectedRolePolicies["test-role"]["test-policy"], currentRolePolicies["test-role"]["test-policy"]))
		})
	}
}

type fakeConfigureExternalAuditStorageClient struct {
	account string
	// rolePolicies is a nested map holding the state of existing roles and
	// their attached policies. Each outer key is a role name, the value is a
	// map of policy names to policy documents.
	rolePolicies map[string]map[string]string
}

func (f *fakeConfigureExternalAuditStorageClient) PutRolePolicy(ctx context.Context, input *iam.PutRolePolicyInput, opts ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error) {
	roleName := aws.ToString(input.RoleName)
	if _, roleExists := f.rolePolicies[roleName]; !roleExists {
		return nil, &iamtypes.NoSuchEntityException{
			Message: aws.String(fmt.Sprintf("role %q does not exist", roleName)),
		}
	}
	if f.rolePolicies[roleName] == nil {
		f.rolePolicies[roleName] = make(map[string]string)
	}
	f.rolePolicies[roleName][aws.ToString(input.PolicyName)] = aws.ToString(input.PolicyDocument)
	return &iam.PutRolePolicyOutput{}, nil
}

func (f *fakeConfigureExternalAuditStorageClient) GetCallerIdentity(ctx context.Context, input *sts.GetCallerIdentityInput, opts ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
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
