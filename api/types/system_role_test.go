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
package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRolesCheck(t *testing.T) {
	tests := []struct {
		roles   SystemRoles
		wantErr bool
	}{
		{roles: []SystemRole{}, wantErr: false},
		{roles: []SystemRole{RoleAuth}, wantErr: false},
		{roles: []SystemRole{RoleAuth, RoleNode, RoleProxy, RoleAdmin, RoleProvisionToken, RoleTrustedCluster, LegacyClusterTokenType, RoleSignup, RoleNop}, wantErr: false},
		{roles: []SystemRole{RoleAuth, RoleNode, RoleAuth}, wantErr: true},
		{roles: []SystemRole{SystemRole("unknown")}, wantErr: true},
	}

	for _, tt := range tests {
		err := tt.roles.Check()
		if (err != nil) != tt.wantErr {
			t.Errorf("%v.Check(): got error %q, want error %v", tt.roles, err, tt.wantErr)
		}
	}
}

func TestRolesEqual(t *testing.T) {
	tests := []struct {
		a, b SystemRoles
		want bool
	}{
		{a: nil, b: nil, want: true},
		{a: []SystemRole{}, b: []SystemRole{}, want: true},
		{a: nil, b: []SystemRole{}, want: true},
		{a: []SystemRole{RoleAuth}, b: []SystemRole{RoleAuth}, want: true},
		{a: []SystemRole{RoleAuth}, b: []SystemRole{RoleAuth, RoleAuth}, want: true},
		{a: []SystemRole{RoleAuth, RoleNode}, b: []SystemRole{RoleNode, RoleAuth}, want: true},
		{a: []SystemRole{RoleAuth}, b: nil, want: false},
		{a: []SystemRole{RoleAuth}, b: []SystemRole{RoleNode}, want: false},
		{a: []SystemRole{RoleAuth, RoleAuth}, b: []SystemRole{RoleAuth, RoleNode}, want: false},
		{a: []SystemRole{RoleAuth, RoleNode}, b: []SystemRole{RoleAuth, RoleAuth}, want: false},
	}

	for _, tt := range tests {
		if got := tt.a.Equals(tt.b); got != tt.want {
			t.Errorf("%v.Equals(%v): got %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestParseTeleportRoles(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		in      string
		out     SystemRoles
		wantErr bool
	}{
		{
			// system role constant
			in:      string(RoleNode),
			out:     SystemRoles{RoleNode},
			wantErr: false,
		},
		{
			// alternate capitalization
			in:      "node",
			out:     SystemRoles{RoleNode},
			wantErr: false,
		},
		{
			// multiple comma-separated roles
			in:      "Auth,Proxy",
			out:     SystemRoles{RoleAuth, RoleProxy},
			wantErr: false,
		},
		{
			// extra whitespace handled properly
			in:      "Auth , Proxy,WindowsDesktop",
			out:     SystemRoles{RoleAuth, RoleProxy, RoleWindowsDesktop},
			wantErr: false,
		},
		{
			// duplicate roles is an error
			in:      "Auth,Auth",
			wantErr: true,
		},
		{
			in:      "Auth, auth",
			wantErr: true,
		},
		{
			// invalid role errors
			in:      "invalidrole",
			wantErr: true,
		},
		{
			// valid + invalid role errors
			in:      "Admin,invalidrole",
			wantErr: true,
		},
	} {
		t.Run(test.in, func(t *testing.T) {
			roles, err := ParseTeleportRoles(test.in)
			if test.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.out, roles)
			}
		})
	}
}
