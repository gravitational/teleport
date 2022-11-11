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

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestExtractCredFromAuthHeader test the extractCredFromAuthHeader function logic.
func TestExtractCredFromAuthHeader(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		expCred *SigV4
		wantErr require.ErrorAssertionFunc
	}{
		{
			name:  "valid header",
			input: "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request, SignedHeaders=host;range;x-amz-date, Signature=fe5f80f77d5fa3beca038a248ff027d0445342fe2855ddc963176630326f1024",
			expCred: &SigV4{
				KeyID:     "AKIAIOSFODNN7EXAMPLE",
				Date:      "20130524",
				Region:    "us-east-1",
				Service:   "s3",
				Signature: "fe5f80f77d5fa3beca038a248ff027d0445342fe2855ddc963176630326f1024",
				SignedHeaders: []string{
					"host",
					"range",
					"x-amz-date",
				},
			},
			wantErr: require.NoError,
		},
		{
			name:  "signed headers section missing",
			input: "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request, Signature=fe5f80f77d5fa3beca038a248ff027d0445342fe2855ddc963176630326f1024",
			expCred: &SigV4{
				KeyID:     "AKIAIOSFODNN7EXAMPLE",
				Date:      "20130524",
				Region:    "us-east-1",
				Service:   "s3",
				Signature: "fe5f80f77d5fa3beca038a248ff027d0445342fe2855ddc963176630326f1024",
			},
			wantErr: require.NoError,
		},
		{
			name:    "credential  section missing",
			input:   "AWS4-HMAC-SHA256 SignedHeaders=host;range;x-amz-date, Signature=fe5f80f77d5fa3beca038a248ff027d0445342fe2855ddc963176630326f1024",
			wantErr: require.Error,
		},
		{
			name:    "invalid format",
			input:   "Credential=AKIAIOSFODNN7EXAMPLE/us-east-1/s3/aws4_request",
			wantErr: require.Error,
		},
		{
			name:    "missing credentials section",
			input:   "AWS4-HMAC-SHA256 SignedHeaders=host",
			wantErr: require.Error,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseSigV4(tc.input)
			tc.wantErr(t, err)
			require.Equal(t, tc.expCred, got)
		})
	}
}

// TestFilterAWSRoles verifies filtering AWS role ARNs by AWS account ID.
func TestFilterAWSRoles(t *testing.T) {
	acc1ARN1 := AWSRole{
		ARN:     "arn:aws:iam::1234567890:role/EC2FullAccess",
		Display: "EC2FullAccess",
		Name:    "EC2FullAccess",
	}
	acc1ARN2 := AWSRole{
		ARN:     "arn:aws:iam::1234567890:role/EC2ReadOnly",
		Display: "EC2ReadOnly",
		Name:    "EC2ReadOnly",
	}
	acc1ARN3 := AWSRole{
		ARN:     "arn:aws:iam::1234567890:role/path/to/customrole",
		Display: "customrole",
		Name:    "path/to/customrole",
	}
	acc2ARN1 := AWSRole{
		ARN:     "arn:aws:iam::0987654321:role/test-role",
		Display: "test-role",
		Name:    "test-role",
	}
	invalidARN := AWSRole{
		ARN: "invalid-arn",
	}
	allARNS := []string{
		acc1ARN1.ARN, acc1ARN2.ARN, acc1ARN3.ARN, acc2ARN1.ARN, invalidARN.ARN,
	}
	tests := []struct {
		name      string
		accountID string
		outARNs   Roles
	}{
		{
			name:      "first account roles",
			accountID: "1234567890",
			outARNs:   Roles{acc1ARN1, acc1ARN2, acc1ARN3},
		},
		{
			name:      "second account roles",
			accountID: "0987654321",
			outARNs:   Roles{acc2ARN1},
		},
		{
			name:      "all roles",
			accountID: "",
			outARNs:   Roles{acc1ARN1, acc1ARN2, acc1ARN3, acc2ARN1},
		},
	}
	for _, test := range tests {
		require.Equal(t, test.outARNs, FilterAWSRoles(allARNS, test.accountID))
	}
}

func TestRoles(t *testing.T) {
	arns := []string{
		"arn:aws:iam::1234567890:role/test-role",
		"arn:aws:iam::1234567890:role/EC2FullAccess",
		"arn:aws:iam::1234567890:role/path/to/EC2FullAccess",
	}
	roles := FilterAWSRoles(arns, "1234567890")
	require.Len(t, roles, 3)

	t.Run("Sort", func(t *testing.T) {
		roles.Sort()
		require.Equal(t, "arn:aws:iam::1234567890:role/EC2FullAccess", roles[0].ARN)
		require.Equal(t, "arn:aws:iam::1234567890:role/path/to/EC2FullAccess", roles[1].ARN)
		require.Equal(t, "arn:aws:iam::1234567890:role/test-role", roles[2].ARN)
	})

	t.Run("FindRoleByARN", func(t *testing.T) {
		t.Run("found", func(t *testing.T) {
			for _, arn := range arns {
				role, found := roles.FindRoleByARN(arn)
				require.True(t, found)
				require.Equal(t, role.ARN, arn)
			}
		})

		t.Run("not found", func(t *testing.T) {
			_, found := roles.FindRoleByARN("arn:aws:iam::1234567889:role/unknown")
			require.False(t, found)
		})
	})

	t.Run("FindRolesByName", func(t *testing.T) {
		t.Run("found zero", func(t *testing.T) {
			rolesWithName := roles.FindRolesByName("unknown")
			require.Empty(t, rolesWithName)
		})

		t.Run("found one", func(t *testing.T) {
			rolesWithName := roles.FindRolesByName("path/to/EC2FullAccess")
			require.Len(t, rolesWithName, 1)
			require.Equal(t, "path/to/EC2FullAccess", rolesWithName[0].Name)
		})

		t.Run("found two", func(t *testing.T) {
			rolesWithName := roles.FindRolesByName("EC2FullAccess")
			require.Len(t, rolesWithName, 2)
			require.Equal(t, "EC2FullAccess", rolesWithName[0].Display)
			require.Equal(t, "EC2FullAccess", rolesWithName[1].Display)
			require.NotEqual(t, rolesWithName[0].ARN, rolesWithName[1].ARN)
		})
	})
}
