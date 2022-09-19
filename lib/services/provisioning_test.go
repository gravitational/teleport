/*
Copyright 2022 Gravitational, Inc.

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

package services

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils/golden"
	"github.com/stretchr/testify/require"
)

func TestMustCreateProvisionToken(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		token := MustCreateProvisionToken(
			"foo",
			types.SystemRoles{types.RoleNop},
			time.Date(2000, 1, 1, 1, 1, 1, 1, time.UTC),
		)
		require.NotNil(t, token)
		require.Equal(t, "foo", token.GetName())
	})
	t.Run("panics on failure", func(t *testing.T) {
		require.Panics(t, func() {
			_ = MustCreateProvisionToken(
				"foo",
				nil,
				time.Date(2000, 1, 1, 1, 1, 1, 1, time.UTC),
			)
		})
	})
}

func TestUnmarshalProvisionToken(t *testing.T) {
	expiry := time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		data []byte
		opts []MarshalOption
		want types.ProvisionToken
	}{
		{
			name: "v1",
			data: []byte(`{"roles":["Nop"],"token":"foo","expires":"1999-11-30T00:00:00Z"}`),
			want: (&types.ProvisionTokenV1{
				Token:   "foo",
				Roles:   types.SystemRoles{types.RoleNop},
				Expires: expiry,
			}).V3(),
		},
		{
			name: "v2",
			data: []byte(`{"kind":"token","version":"v2","metadata":{"name":"foo","expires":"1999-11-30T00:00:00Z"},"spec":{"roles":["Nop"],"join_method":"token"}}`),
			want: &types.ProvisionTokenV2{
				Kind:    types.KindToken,
				Version: types.V2,
				Metadata: types.Metadata{
					Name:      "foo",
					Expires:   &expiry,
					Namespace: defaults.Namespace,
				},
				Spec: types.ProvisionTokenSpecV2{
					Roles:      types.SystemRoles{types.RoleNop},
					JoinMethod: types.JoinMethodToken,
				},
			},
		},
		{
			name: "v3",
			data: []byte(`{"kind":"token","version":"v3","metadata":{"name":"foo","expires":"1999-11-30T00:00:00Z"},"spec":{"roles":["Nop"],"join_method":"token"}}`),
			want: MustCreateProvisionToken(
				"foo",
				types.SystemRoles{types.RoleNop},
				expiry,
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UnmarshalProvisionToken(tt.data, tt.opts...)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestMarshalProvisionToken(t *testing.T) {
	v3 := MustCreateProvisionToken(
		"foo",
		types.SystemRoles{types.RoleNop},
		time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC),
	)
	v3.SetResourceID(1337)
	v2, _ := v3.V2()

	tests := []struct {
		name  string
		token types.ProvisionToken
		opts  []MarshalOption
	}{
		{
			name:  "v2",
			token: v2,
		},
		{
			name:  "v3",
			token: v3,
		},
		{
			name:  "v2 - PreserveResourceID",
			token: v2,
			opts:  []MarshalOption{PreserveResourceID()},
		},
		{
			name:  "v3 - PreserveResourceID",
			token: v3,
			opts:  []MarshalOption{PreserveResourceID()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := MarshalProvisionToken(tt.token, tt.opts...)
			require.NoError(t, err)
			if golden.ShouldSet() {
				golden.Set(t, bytes)
			}
			require.Equal(t, golden.Get(t), bytes)
		})
	}
}
