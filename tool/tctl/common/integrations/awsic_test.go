// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package integrations

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

func TestWriteAWSICOutput(t *testing.T) {
	t.Parallel()

	assignments := []*identitycenterv1.AccountAssignment{
		newICAccountAssignment("111111111111", "Production", "AdminAccess", "arn:aws:sso:::permissionSet/abc/ps-aaaa"),
		newICAccountAssignment("111111111111", "Production", "ReadOnly", "arn:aws:sso:::permissionSet/abc/ps-bbbb"),
		newICAccountAssignment("222222222222", "Staging", "ReadOnly", "arn:aws:sso:::permissionSet/abc/ps-bbbb"),
	}

	tests := []struct {
		name   string
		format string
	}{
		{
			name:   "text",
			format: "", // default text
		},
		{
			name:   "json",
			format: teleport.JSON,
		},
		{
			name:   "yaml",
			format: teleport.YAML,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			c := &Command{Stdout: &buf, awsicAccountsLsFormat: tt.format}
			require.NoError(t, c.writeAWSICOutput(assignments))
			if golden.ShouldSet() {
				golden.Set(t, buf.Bytes())
			}
			require.Equal(t, string(golden.Get(t)), buf.String())
		})
	}
}

func newICAccountAssignment(accountID, accountName, permSetName, permSetARN string) *identitycenterv1.AccountAssignment {
	return identitycenterv1.AccountAssignment_builder{
		Spec: identitycenterv1.AccountAssignmentSpec_builder{
			AccountId:   accountID,
			AccountName: accountName,
			PermissionSet: identitycenterv1.PermissionSetInfo_builder{
				Name: permSetName,
				Arn:  permSetARN,
			}.Build(),
		}.Build(),
	}.Build()
}
