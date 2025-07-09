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
			name:  "valid header without spaces",
			input: "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request,SignedHeaders=host;x-amz-content-sha256;x-amz-date,Signature=fe5f80f77d5fa3beca038a248ff027d0445342fe2855ddc963176630326f1024",
			expCred: &SigV4{
				KeyID:     "AKIAIOSFODNN7EXAMPLE",
				Date:      "20130524",
				Region:    "us-east-1",
				Service:   "s3",
				Signature: "fe5f80f77d5fa3beca038a248ff027d0445342fe2855ddc963176630326f1024",
				SignedHeaders: []string{
					"host",
					"x-amz-content-sha256",
					"x-amz-date",
				},
			},
			wantErr: require.NoError,
		},
		{
			name:  "valid with empty list of signed headers",
			input: "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request,SignedHeaders=,Signature=fe5f80f77d5fa3beca038a248ff027d0445342fe2855ddc963176630326f1024",
			expCred: &SigV4{
				KeyID:         "AKIAIOSFODNN7EXAMPLE",
				Date:          "20130524",
				Region:        "us-east-1",
				Service:       "s3",
				Signature:     "fe5f80f77d5fa3beca038a248ff027d0445342fe2855ddc963176630326f1024",
				SignedHeaders: nil,
			},
			wantErr: require.NoError,
		},
		{
			name:    "signed headers section missing",
			input:   "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request, Signature=fe5f80f77d5fa3beca038a248ff027d0445342fe2855ddc963176630326f1024",
			wantErr: require.Error,
		},
		{
			name:    "credential section missing",
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
	acc1ARN1 := Role{
		ARN:       "arn:aws:iam::123456789012:role/EC2FullAccess",
		Display:   "EC2FullAccess",
		Name:      "EC2FullAccess",
		AccountID: "123456789012",
	}
	acc1ARN2 := Role{
		ARN:       "arn:aws:iam::123456789012:role/EC2ReadOnly",
		Display:   "EC2ReadOnly",
		Name:      "EC2ReadOnly",
		AccountID: "123456789012",
	}
	acc1ARN3 := Role{
		ARN:       "arn:aws:iam::123456789012:role/path/to/customrole",
		Display:   "customrole",
		Name:      "path/to/customrole",
		AccountID: "123456789012",
	}
	acc2ARN1 := Role{
		ARN:       "arn:aws:iam::210987654321:role/test-role",
		Display:   "test-role",
		Name:      "test-role",
		AccountID: "210987654321",
	}
	invalidARN := Role{
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
			accountID: "123456789012",
			outARNs:   Roles{acc1ARN1, acc1ARN2, acc1ARN3},
		},
		{
			name:      "second account roles",
			accountID: "210987654321",
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
		"arn:aws:iam::123456789012:role/test-role",
		"arn:aws:iam::123456789012:role/EC2FullAccess",
		"arn:aws:iam::123456789012:role/path/to/EC2FullAccess",
	}
	roles := FilterAWSRoles(arns, "123456789012")
	require.Len(t, roles, 3)

	t.Run("Sort", func(t *testing.T) {
		roles.Sort()
		require.Equal(t, "arn:aws:iam::123456789012:role/EC2FullAccess", roles[0].ARN)
		require.Equal(t, "arn:aws:iam::123456789012:role/path/to/EC2FullAccess", roles[1].ARN)
		require.Equal(t, "arn:aws:iam::123456789012:role/test-role", roles[2].ARN)
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
			_, found := roles.FindRoleByARN("arn:aws:iam::123456788912:role/unknown")
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

func TestValidateRoleARNAndExtractRoleName(t *testing.T) {
	tests := []struct {
		name           string
		inputARN       string
		inputPartition string
		inputAccountID string
		wantRoleName   string
		wantError      bool
	}{
		{
			name:           "success",
			inputARN:       "arn:aws:iam::123456789012:role/role-name",
			inputPartition: "aws",
			inputAccountID: "123456789012",
			wantRoleName:   "role-name",
		},
		{
			name:           "invalid arn",
			inputARN:       "arn::::aws:iam::123456789012:role/role-name",
			inputPartition: "aws",
			inputAccountID: "123456789012",
			wantError:      true,
		},
		{
			name:           "invalid partition",
			inputARN:       "arn:aws:iam::123456789012:role/role-name",
			inputPartition: "aws-cn",
			inputAccountID: "123456789012",
			wantError:      true,
		},
		{
			name:           "invalid account ID",
			inputARN:       "arn:aws:iam::123456789012:role/role-name",
			inputPartition: "aws",
			inputAccountID: "123456789000",
			wantError:      true,
		},
		{
			name:           "not role arn",
			inputARN:       "arn:aws:iam::123456789012:user/username",
			inputPartition: "aws",
			inputAccountID: "123456789012",
			wantError:      true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualRoleName, err := ValidateRoleARNAndExtractRoleName(test.inputARN, test.inputPartition, test.inputAccountID)
			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.wantRoleName, actualRoleName)
			}
		})
	}
}

func TestParseRoleARN(t *testing.T) {
	tests := map[string]struct {
		arn             string
		wantErrContains string
	}{
		"valid role arn": {
			arn: "arn:aws:iam::123456789012:role/test-role",
		},
		"valid sso role arn": {
			arn: "arn:aws:iam::123456789012:role/aws-reserved/sso.amazonaws.com/us-west-2/AWSReservedSSO_AWSPowerUserAccess_xxxxxxxxx",
		},
		"valid service role arn": {
			arn: "arn:aws:iam::123456789012:role/aws-service-role/redshift.amazonaws.com/AWSServiceRoleForRedshift",
		},
		"arn fails to parse": {
			arn:             "foobar",
			wantErrContains: "invalid AWS ARN",
		},
		"sts arn is not iam": {
			arn:             "arn:aws:sts::123456789012:federated-user/Alice",
			wantErrContains: "not an AWS IAM role",
		},
		"iam arn is not a role": {
			arn:             "arn:aws:iam::123456789012:user/test-user",
			wantErrContains: "not an AWS IAM role",
		},
		"iam role arn is missing role name section": {
			arn:             "arn:aws:iam::123456789012:role",
			wantErrContains: "missing AWS IAM role name",
		},
		"iam role arn is missing role name": {
			arn:             "arn:aws:iam::123456789012:role/",
			wantErrContains: "missing AWS IAM role name",
		},
		"service role arn is missing role name": {
			arn:             "arn:aws:iam::123456789012:role/aws-service-role/redshift.amazonaws.com/",
			wantErrContains: "missing AWS IAM role name",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := ParseRoleARN(tt.arn)
			if tt.wantErrContains != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.wantErrContains)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
		})
	}
}

func TestBuildRoleARN(t *testing.T) {
	tests := map[string]struct {
		user            string
		region          string
		accountID       string
		wantErrContains string
		wantARN         string
	}{
		"valid role arn in correct partition and account": {
			user:      "arn:aws:iam::123456789012:role/test-role",
			region:    "us-west-1",
			accountID: "123456789012",
			wantARN:   "arn:aws:iam::123456789012:role/test-role",
		},
		"valid role arn in correct account and default partition": {
			user:      "arn:aws:iam::123456789012:role/test-role",
			region:    "",
			accountID: "123456789012",
			wantARN:   "arn:aws:iam::123456789012:role/test-role",
		},
		"valid role arn in default partition and account": {
			user:      "arn:aws:iam::123456789012:role/test-role",
			region:    "",
			accountID: "",
			wantARN:   "arn:aws:iam::123456789012:role/test-role",
		},
		"role name with prefix in default partition and account": {
			user:      "role/test-role",
			region:    "",
			accountID: "123456789012",
			wantARN:   "arn:aws:iam::123456789012:role/test-role",
		},
		"role name in default partition and account": {
			user:      "test-role",
			region:    "",
			accountID: "123456789012",
			wantARN:   "arn:aws:iam::123456789012:role/test-role",
		},
		"role name in china partition and account": {
			user:      "test-role",
			region:    "cn-north-1",
			accountID: "123456789012",
			wantARN:   "arn:aws-cn:iam::123456789012:role/test-role",
		},
		"valid ARN is not an IAM role ARN": {
			user:            "arn:aws:iam::123456789012:user/test-user",
			region:          "",
			accountID:       "",
			wantErrContains: "not an AWS IAM role",
		},
		"valid role arn in different partition": {
			user:            "arn:aws-cn:iam::123456789012:role/test-role",
			region:          "us-west-1",
			accountID:       "",
			wantErrContains: `expected AWS partition "aws" but got "aws-cn"`,
		},
		"valid role arn in different account": {
			user:            "arn:aws:iam::123456789012:role/test-role",
			region:          "us-west-1",
			accountID:       "111222333444",
			wantErrContains: `expected AWS account ID "111222333444" but got "123456789012"`,
		},
		"role name with invalid account characters": {
			user:            "test-role",
			region:          "",
			accountID:       "12345678901f",
			wantErrContains: "must be 12-digit",
		},
		"role name with invalid account id length": {
			user:            "test-role",
			region:          "",
			accountID:       "1234567890123",
			wantErrContains: "must be 12-digit",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := BuildRoleARN(tt.user, tt.region, tt.accountID)
			if tt.wantErrContains != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.wantErrContains)
				return
			}
			require.NoError(t, err)
			require.NotEmpty(t, got)
			require.Equal(t, tt.wantARN, got)
		})
	}
}

func TestIsRoleARN(t *testing.T) {
	for name, tt := range map[string]struct {
		arn           string
		expectedValue bool
	}{
		"valid full arn":      {"arn:aws:iam::123456789012:role/role-name", true},
		"valid partial arn":   {"role/role-name", true},
		"valid user arn":      {"arn:aws:iam::123456789012:user/user-name", false},
		"invalid arn":         {"arn:aws:iam:::123456789012:role/role-name", false},
		"invalid partial arn": {"user/user-name", false},
		"invalid value":       {"role-name", false},
	} {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tt.expectedValue, IsRoleARN(tt.arn))
		})
	}
}

func TestIsUserARN(t *testing.T) {
	for name, tt := range map[string]struct {
		arn           string
		expectedValue bool
	}{
		"valid full arn":      {"arn:aws:iam::123456789012:user/user-name", true},
		"valid partial arn":   {"user/user-name", true},
		"valid user arn":      {"arn:aws:iam::123456789012:role/role-name", false},
		"invalid arn":         {"arn:aws:iam:::123456789012:user/user-name", false},
		"invalid partial arn": {"role/role-name", false},
		"invalid value":       {"user-name", false},
	} {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tt.expectedValue, IsUserARN(tt.arn))
		})
	}
}

func FuzzParseSigV4(f *testing.F) {
	f.Add("")
	f.Add("Authorization: AWS4-HMAC-SHA256 " +
		"Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request, " +
		"SignedHeaders=host;range;x-amz-date, " +
		"Signature=fe5f80f77d5fa3beca038a248ff027d0445342fe2855ddc963176630326f1024")

	f.Fuzz(func(t *testing.T, str string) {
		require.NotPanics(t, func() {
			_, _ = ParseSigV4(str)
		})
	})
}

func TestResourceARN(t *testing.T) {
	for _, tt := range []struct {
		name         string
		resourceType string
		partition    string
		accountID    string
		resourceName string
		expected     string
	}{
		{
			name:         "role",
			resourceType: "role",
			partition:    "aws",
			accountID:    "123456789012",
			resourceName: "MyRole",
			expected:     "arn:aws:iam::123456789012:role/MyRole",
		},
		{
			name:         "policy",
			resourceType: "policy",
			partition:    "aws",
			accountID:    "123456789012",
			resourceName: "MyPolicy",
			expected:     "arn:aws:iam::123456789012:policy/MyPolicy",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.resourceType {
			case "role":
				require.Equal(t, tt.expected, RoleARN(tt.partition, tt.accountID, tt.resourceName))
			case "policy":
				require.Equal(t, tt.expected, PolicyARN(tt.partition, tt.accountID, tt.resourceName))
			}
		})
	}
}

func TestMaybeHashRoleSessionName(t *testing.T) {
	for _, tt := range []struct {
		name     string
		role     string
		expected string
	}{
		{
			name:     "role session name not hashed, less than 64 characters",
			role:     "MyRole",
			expected: "MyRole",
		},
		{
			name:     "role session name not hashed, exactly 64 characters",
			role:     "Role123456789012345678901234567890123456789012345678901234567890",
			expected: "Role123456789012345678901234567890123456789012345678901234567890",
		},
		{
			name:     "role session name hashed, longer than 64 characters",
			role:     "remote-raimundo.oliveira@abigcompany.com-teleport.abigcompany.com",
			expected: "remote-raimundo.oliveira@abigcompany.com-telepo-8fe1f87e599b043e",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			actual := MaybeHashRoleSessionName(tt.role)
			require.Equal(t, tt.expected, actual)
			require.LessOrEqual(t, len(actual), MaxRoleSessionNameLength)
		})
	}
}
