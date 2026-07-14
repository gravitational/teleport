/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package common

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestParseResourceValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		values    []string
		assertErr require.ErrorAssertionFunc
		want      []types.ResourceAccessID
	}{
		{
			name:      "plain node unconstrained",
			values:    []string{"/main/node/web-1"},
			assertErr: require.NoError,
			want: []types.ResourceAccessID{
				{Id: types.ResourceID{ClusterName: "main", Kind: types.KindNode, Name: "web-1"}},
			},
		},
		{
			name:      "inline single login",
			values:    []string{"/main/node/web-1|logins=root"},
			assertErr: require.NoError,
			want: []types.ResourceAccessID{
				sshRAID("main", "web-1", "root"),
			},
		},
		{
			name:      "inline multiple logins",
			values:    []string{"/main/node/web-1|logins=root,admin"},
			assertErr: require.NoError,
			want: []types.ResourceAccessID{
				sshRAID("main", "web-1", "root", "admin"),
			},
		},
		{
			name:      "inline role arns on app",
			values:    []string{"/main/app/console|role_arns=arn:aws:iam::123:role/Admin"},
			assertErr: require.NoError,
			want: []types.ResourceAccessID{
				awsRAID("main", "console", "arn:aws:iam::123:role/Admin"),
			},
		},
		{
			name:      "duplicate keys merged",
			values:    []string{"/main/node/web-1|logins=root|logins=admin"},
			assertErr: require.NoError,
			want: []types.ResourceAccessID{
				sshRAID("main", "web-1", "root", "admin"),
			},
		},
		{
			name:      "resource name with pipe but no key marker stays in name",
			values:    []string{"/main/node/we|rd-name"},
			assertErr: require.NoError,
			want: []types.ResourceAccessID{
				{Id: types.ResourceID{ClusterName: "main", Kind: types.KindNode, Name: "we|rd-name"}},
			},
		},
		{
			name:      "unrecognized key after a real key stays an error",
			values:    []string{"/main/node/web-1|logins=root|frobnicate=x"},
			assertErr: require.Error,
		},
		{
			name:      "unknown constraint key is rejected",
			values:    []string{"/main/db/postgres|db_users=admin"},
			assertErr: require.Error,
		},
		{
			name:      "pipe in resource name before a real constraint",
			values:    []string{"/main/node/web|1|logins=root"},
			assertErr: require.NoError,
			want: []types.ResourceAccessID{
				sshRAID("main", "web|1", "root"),
			},
		},
		{
			name:      "two known keys cannot combine",
			values:    []string{"/main/node/web-1|logins=root|role_arns=arn:aws:iam::123:role/Admin"},
			assertErr: require.Error,
		},
		{
			name:      "key not applicable to kind is rejected",
			values:    []string{"/main/app/console|logins=root"},
			assertErr: require.Error,
		},
		{
			name:      "empty value is rejected",
			values:    []string{"/main/node/web-1|logins="},
			assertErr: require.Error,
		},
		{
			name:      "empty value among values is rejected",
			values:    []string{"/main/node/web-1|logins=root,,admin"},
			assertErr: require.Error,
		},
		{
			name:      "json form ssh",
			values:    []string{`{"id":{"cluster":"main","kind":"node","name":"web-1"},"constraints":{"version":"v1","ssh":{"logins":["root","admin"]}}}`},
			assertErr: require.NoError,
			want: []types.ResourceAccessID{
				sshRAID("main", "web-1", "root", "admin"),
			},
		},
		{
			name:      "json form kind mismatch is rejected",
			values:    []string{`{"id":{"cluster":"main","kind":"app","name":"console"},"constraints":{"version":"v1","ssh":{"logins":["root"]}}}`},
			assertErr: require.Error,
		},
		{
			name:      "malformed json is rejected",
			values:    []string{`{"id":`},
			assertErr: require.Error,
		},
		{
			name:      "invalid resource id is rejected",
			values:    []string{"not-a-resource-id"},
			assertErr: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseResourceValues(tt.values)
			tt.assertErr(t, err)
			if err != nil {
				return
			}
			require.Equal(t, tt.want, got)
		})
	}
}

func TestSplitInlineConstraints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		value      string
		wantID     string
		wantSuffix string
	}{
		{"no constraints", "/main/node/web-1", "/main/node/web-1", ""},
		{"single known key", "/main/node/web-1|logins=root,admin", "/main/node/web-1", "logins=root,admin"},
		{"pipe in name, no constraint", "/main/node/we|rd-name", "/main/node/we|rd-name", ""},
		{"pipe in name before constraint", "/main/node/web|1|logins=root", "/main/node/web|1", "logins=root"},
		// Forward compatibility: keys this build has never seen still split off
		// cleanly instead of being folded into the resource name, so an older
		// client gives a clear "unknown key" error rather than a mangled ID.
		{"unknown future keys still split", "/main/db/pg|db_users=alice|db_names=sales", "/main/db/pg", "db_users=alice|db_names=sales"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotID, gotSuffix := splitInlineConstraints(tt.value)
			require.Equal(t, tt.wantID, gotID)
			require.Equal(t, tt.wantSuffix, gotSuffix)
		})
	}
}

func TestParseResourceAccessIDListJSON(t *testing.T) {
	t.Parallel()

	t.Run("mixed constrained and plain", func(t *testing.T) {
		t.Parallel()
		data := []byte(`{"resources":[
			{"id":{"cluster":"main","kind":"node","name":"web-1"},"constraints":{"version":"v1","ssh":{"logins":["root"]}}},
			{"id":{"cluster":"main","kind":"node","name":"web-2"}}
		]}`)
		got, err := ParseResourceAccessIDListJSON(data)
		require.NoError(t, err)
		require.Equal(t, []types.ResourceAccessID{
			sshRAID("main", "web-1", "root"),
			{Id: types.ResourceID{ClusterName: "main", Kind: types.KindNode, Name: "web-2"}},
		}, got)
	})

	t.Run("kind mismatch in list is rejected", func(t *testing.T) {
		t.Parallel()
		data := []byte(`{"resources":[
			{"id":{"cluster":"main","kind":"app","name":"console"},"constraints":{"version":"v1","ssh":{"logins":["root"]}}}
		]}`)
		_, err := ParseResourceAccessIDListJSON(data)
		require.Error(t, err)
	})
}

func TestParseResourceAccessIDListFile(t *testing.T) {
	t.Parallel()

	const listJSON = `{"resources":[{"id":{"cluster":"main","kind":"node","name":"web-1"},"constraints":{"version":"v1","ssh":{"logins":["root"]}}}]}`
	want := []types.ResourceAccessID{sshRAID("main", "web-1", "root")}

	t.Run("from file", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "resources.json")
		require.NoError(t, os.WriteFile(path, []byte(listJSON), 0o600))
		got, err := ParseResourceAccessIDListFile(path, nil)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})

	t.Run("from stdin", func(t *testing.T) {
		t.Parallel()
		got, err := ParseResourceAccessIDListFile("-", strings.NewReader(listJSON))
		require.NoError(t, err)
		require.Equal(t, want, got)
	})

	t.Run("missing file", func(t *testing.T) {
		t.Parallel()
		_, err := ParseResourceAccessIDListFile(filepath.Join(t.TempDir(), "nope.json"), nil)
		require.Error(t, err)
	})

	t.Run("invalid json", func(t *testing.T) {
		t.Parallel()
		_, err := ParseResourceAccessIDListFile("-", strings.NewReader("not json"))
		require.Error(t, err)
	})
}

func sshRAID(cluster, name string, logins ...string) types.ResourceAccessID {
	return types.ResourceAccessID{
		Id: types.ResourceID{ClusterName: cluster, Kind: types.KindNode, Name: name},
		Constraints: &types.ResourceConstraints{
			Version: types.ResourceConstraintVersionV1,
			Details: &types.ResourceConstraints_Ssh{Ssh: &types.SSHResourceConstraints{Logins: logins}},
		},
	}
}

func awsRAID(cluster, name string, arns ...string) types.ResourceAccessID {
	return types.ResourceAccessID{
		Id: types.ResourceID{ClusterName: cluster, Kind: types.KindApp, Name: name},
		Constraints: &types.ResourceConstraints{
			Version: types.ResourceConstraintVersionV1,
			Details: &types.ResourceConstraints_AwsConsole{AwsConsole: &types.AWSConsoleResourceConstraints{RoleArns: arns}},
		},
	}
}

func TestParseResourceValuesEscaping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		want    types.ResourceAccessID
		wantErr string
	}{
		{
			name:  "escaped comma in ARN value",
			value: `/main/app/aws-console|role_arns=arn:aws:iam::123:role/a\,b,arn:aws:iam::123:role/c`,
			want:  awsRAID("main", "aws-console", "arn:aws:iam::123:role/a,b", "arn:aws:iam::123:role/c"),
		},
		{
			name:  "escaped backslash in value",
			value: `/main/node/web-1|logins=a\\b`,
			want:  sshRAID("main", "web-1", `a\b`),
		},
		{
			name:  "literal equals in value needs no escape",
			value: `/main/app/aws-console|role_arns=arn:aws:iam::123:role/a=b`,
			want:  awsRAID("main", "aws-console", "arn:aws:iam::123:role/a=b"),
		},
		{
			name:    "dangling escape rejected",
			value:   `/main/node/web-1|logins=root\`,
			wantErr: "dangling escape",
		},
		{
			name:    "unknown escape rejected",
			value:   `/main/node/web-1|logins=ro\ot`,
			wantErr: "unsupported escape",
		},
		{
			name:    "planned key gets not-yet-supported error",
			value:   `/main/db/postgres|db_users=admin`,
			wantErr: "not yet supported",
		},
		{
			name:    "unknown key stays unknown",
			value:   `/main/node/web-1|shoe_size=44`,
			wantErr: "unknown constraint key",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseResourceValues([]string{tt.value})
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Len(t, got, 1)
			require.Empty(t, cmp.Diff(tt.want, got[0]))
		})
	}
}
