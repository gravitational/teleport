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
