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

package main

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/client"
)

func Test_getGCPServiceAccountFromFlags(t *testing.T) {
	tests := []struct {
		name                    string
		requestedServiceAccount string
		profileAccounts         []string
		want                    string
		wantErr                 require.ErrorAssertionFunc
	}{
		{
			name:                    "no flag, use default service account",
			requestedServiceAccount: "",
			profileAccounts:         []string{"default"},
			want:                    "default",
			wantErr:                 require.NoError,
		},
		{
			name:                    "no flag, multiple possible service accounts",
			requestedServiceAccount: "",
			profileAccounts:         []string{"id1", "id2"},
			want:                    "",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "multiple GCP service accounts available, choose one with --gcp-service-account flag")
			},
		},
		{
			name:                    "no flag, no service accounts",
			requestedServiceAccount: "",
			profileAccounts:         []string{},
			want:                    "",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "no GCP service accounts available, check your permissions")
			},
		},

		{
			name:                    "exact match, one option",
			requestedServiceAccount: "id1",
			profileAccounts:         []string{"id1"},
			want:                    "id1",
			wantErr:                 require.NoError,
		},
		{
			name:                    "exact match, multiple options",
			requestedServiceAccount: "id1",
			profileAccounts:         []string{"id1", "id2"},
			want:                    "id1",
			wantErr:                 require.NoError,
		},
		{
			name:                    "no match, multiple options",
			requestedServiceAccount: "id3",
			profileAccounts:         []string{"id1", "id2"},
			want:                    "",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "failed to find the service account matching \"id3\"")
			},
		},

		{
			name:                    "prefix match, one option",
			requestedServiceAccount: "id1",
			profileAccounts:         []string{"id1@example-123456.iam.gserviceaccount.com"},
			want:                    "id1@example-123456.iam.gserviceaccount.com",
			wantErr:                 require.NoError,
		},
		{
			name:                    "prefix match, multiple options",
			requestedServiceAccount: "id1",
			profileAccounts: []string{
				"id1@example-123456.iam.gserviceaccount.com",
				"id2@example-123456.iam.gserviceaccount.com",
			},
			want:    "id1@example-123456.iam.gserviceaccount.com",
			wantErr: require.NoError,
		},
		{
			name:                    "ambiguous prefix match",
			requestedServiceAccount: "id1",
			profileAccounts: []string{
				"id1@example-123456.iam.gserviceaccount.com",
				"id1@example-777777.iam.gserviceaccount.com",
			},
			want: "",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "provided service account \"id1\" is ambiguous, please specify full service account name")
			},
		},

		{
			name:                    "no match, multiple options",
			requestedServiceAccount: "id3",
			profileAccounts: []string{
				"id1@example-123456.iam.gserviceaccount.com",
				"id2@example-123456.iam.gserviceaccount.com",
				"idX@example-777777.iam.gserviceaccount.com",
			},
			want: "",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "failed to find the service account matching \"id3\"")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := getGCPServiceAccountFromFlags(&CLIConf{GCPServiceAccount: tt.requestedServiceAccount}, &client.ProfileStatus{GCPServiceAccounts: tt.profileAccounts})
			require.Equal(t, tt.want, result)
			tt.wantErr(t, err)
		})
	}
}

func Test_formatGCPServiceAccounts(t *testing.T) {
	tests := []struct {
		name     string
		accounts []string
		want     string
	}{
		{
			name:     "empty",
			accounts: nil,
			want:     "",
		},
		{
			name: "multiple, unsorted",
			accounts: []string{
				"test-3@example-123456.iam.gserviceaccount.com",
				"test-2@example-123456.iam.gserviceaccount.com",
				"test-1@example-123456.iam.gserviceaccount.com",
				"test-0@example-100200.iam.gserviceaccount.com",
				"test-0@other-999999.iam.gserviceaccount.com",
			},
			want: `Available GCP service accounts                
--------------------------------------------- 
test-0@example-100200.iam.gserviceaccount.com 
test-1@example-123456.iam.gserviceaccount.com 
test-2@example-123456.iam.gserviceaccount.com 
test-3@example-123456.iam.gserviceaccount.com 
test-0@other-999999.iam.gserviceaccount.com   
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, formatGCPServiceAccounts(tt.accounts))
		})
	}
}

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
