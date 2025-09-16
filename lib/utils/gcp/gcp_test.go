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

package gcp

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSortedGCPServiceAccounts(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "empty",
			args: nil,
			want: nil,
		},
		{
			name: "unsorted accounts",
			args: []string{
				"test-3@example-123456.iam.gserviceaccount.com",
				"test-2@example-123456.iam.gserviceaccount.com",
				"test-1@example-123456.iam.gserviceaccount.com",
				"test-0@example-100200.iam.gserviceaccount.com",
				"test-0@other-999999.iam.gserviceaccount.com",
			},
			want: []string{
				"test-0@example-100200.iam.gserviceaccount.com",
				"test-1@example-123456.iam.gserviceaccount.com",
				"test-2@example-123456.iam.gserviceaccount.com",
				"test-3@example-123456.iam.gserviceaccount.com",
				"test-0@other-999999.iam.gserviceaccount.com",
			},
		},
		{
			name: "invalid accounts",
			args: []string{
				"",
				"@",
				"@@@",
				"test-3_example-123456.iam.gserviceaccount.com",
				"test-2_example-123456.iam.gserviceaccount.com",
				"test-1_example-123456.iam.gserviceaccount.com",
				"test-0_example-100200.iam.gserviceaccount.com",
				"test-0_other-999999.iam.gserviceaccount.com",
			},
			want: []string{
				"",
				"@",
				"test-0_example-100200.iam.gserviceaccount.com",
				"test-0_other-999999.iam.gserviceaccount.com",
				"test-1_example-123456.iam.gserviceaccount.com",
				"test-2_example-123456.iam.gserviceaccount.com",
				"test-3_example-123456.iam.gserviceaccount.com",
				"@@@",
			},
		},
		{
			name: "mixed invalid and valid accounts",
			args: []string{
				"",
				"@",
				"@@@",
				"test-3_example-123456.iam.gserviceaccount.com",
				"test-2_example-123456.iam.gserviceaccount.com",
				"test-3@example-123456.iam.gserviceaccount.com",
				"test-2@example-123456.iam.gserviceaccount.com",
				"test-1_example-123456.iam.gserviceaccount.com",
				"test-0@example-100200.iam.gserviceaccount.com",
				"test-0_other-999999.iam.gserviceaccount.com",
			},
			want: []string{
				"",
				"@",
				"test-0_other-999999.iam.gserviceaccount.com",
				"test-1_example-123456.iam.gserviceaccount.com",
				"test-2_example-123456.iam.gserviceaccount.com",
				"test-3_example-123456.iam.gserviceaccount.com",
				"@@@",
				"test-0@example-100200.iam.gserviceaccount.com",
				"test-2@example-123456.iam.gserviceaccount.com",
				"test-3@example-123456.iam.gserviceaccount.com",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acc := SortedGCPServiceAccounts(tt.args)
			sort.Sort(acc)
			require.Equal(t, tt.want, []string(acc))
		})
	}
}

func TestProjectIDFromServiceAccountName(t *testing.T) {
	tests := []struct {
		name           string
		serviceAccount string
		want           string
		wantErr        require.ErrorAssertionFunc
	}{
		{
			name:           "valid service account",
			serviceAccount: "test@myproject-123456.iam.gserviceaccount.com",
			want:           "myproject-123456",
			wantErr:        require.NoError,
		},
		{
			name:           "empty string",
			serviceAccount: "",
			want:           "",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "invalid service account format: empty string received")
			},
		},
		{
			name:           "missing @",
			serviceAccount: "test",
			want:           "",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "invalid service account format: missing @")
			},
		},
		{
			name:           "missing domain after @",
			serviceAccount: "test@",
			want:           "",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "invalid service account format: missing <project-id>.iam.gserviceaccount.com after @")
			},
		},
		{
			name:           "missing user before @",
			serviceAccount: "@project",
			want:           "",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "invalid service account format: empty user")
			},
		},
		{
			name:           "missing domain",
			serviceAccount: "test@myproject-123456",
			want:           "",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "invalid service account format: missing <project-id>.iam.gserviceaccount.com after @")
			},
		},
		{
			name:           "wrong domain suffix",
			serviceAccount: "test@myproject-123456.iam.gserviceaccount",
			want:           "",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "invalid service account format: expected suffix \"iam.gserviceaccount.com\", got \"iam.gserviceaccount\"")
			},
		},
		{
			name:           "missing project id",
			serviceAccount: "test@.iam.gserviceaccount.com",
			want:           "",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "invalid service account format: missing project ID")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ProjectIDFromServiceAccountName(tt.serviceAccount)
			require.Equal(t, tt.want, got)
			tt.wantErr(t, err)
		})
	}
}

func TestGetServiceAccountFromCredentialsJSON(t *testing.T) {
	tests := []struct {
		name               string
		credentialsJSON    []byte
		checkError         require.ErrorAssertionFunc
		wantServiceAccount string
	}{
		{
			name:               "service_account credentials",
			credentialsJSON:    []byte(fakeServiceAccountCredentialsJSON),
			checkError:         require.NoError,
			wantServiceAccount: "my-service-account@teleport-example-123456.iam.gserviceaccount.com",
		},
		{
			name:               "external_account credentials with sa impersonation",
			credentialsJSON:    []byte(fakeExternalAccountCredentialsJSON),
			checkError:         require.NoError,
			wantServiceAccount: "my-service-account@teleport-example-987654.iam.gserviceaccount.com",
		},
		{
			name:            "unknown credentials",
			credentialsJSON: []byte(`{}`),
			checkError:      require.Error,
		},
		{
			name:            "bad json",
			credentialsJSON: []byte(`{}`),
			checkError:      require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sa, err := GetServiceAccountFromCredentialsJSON(tc.credentialsJSON)
			tc.checkError(t, err)
			require.Equal(t, tc.wantServiceAccount, sa)
		})
	}
}

const (
	fakeServiceAccountCredentialsJSON = `{
  "type": "service_account",
  "project_id": "teleport-example-123456",
  "private_key_id": "1234569890abcdef1234567890abcdef12345678",
  "private_key": "fake-private-key",
  "client_email": "my-service-account@teleport-example-123456.iam.gserviceaccount.com",
  "client_id": "111111111111111111111",
  "auth_uri": "https://accounts.google.com/o/oauth2/auth",
  "token_uri": "https://oauth2.googleapis.com/token",
  "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
  "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/my-service-account%40teleport-example-123456.iam.gserviceaccount.com",
  "universe_domain": "googleapis.com"
}`
	fakeExternalAccountCredentialsJSON = `{
  "type": "external_account",
  "audience": "//iam.googleapis.com/projects/111111111111/locations/global/workloadIdentityPools/my-identity-pool/providers/my-provider",
  "subject_token_type": "urn:ietf:params:aws:token-type:aws4_request",
  "service_account_impersonation_url": "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/my-service-account@teleport-example-987654.iam.gserviceaccount.com:generateAccessToken",
  "token_url": "https://sts.googleapis.com/v1/token",
  "credential_source": {
    "environment_id": "aws1",
    "region_url": "http://169.254.169.254/latest/meta-data/placement/availability-zone",
    "url": "http://169.254.169.254/latest/meta-data/iam/security-credentials",
    "regional_cred_verification_url": "https://sts.{region}.amazonaws.com?Action=GetCallerIdentity&Version=2011-06-15",
    "imdsv2_session_token_url": "http://169.254.169.254/latest/api/token"
  }
}`
)
