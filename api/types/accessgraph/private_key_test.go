/*
Copyright 2024 Gravitational, Inc.

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

package accessgraph

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	accessgraphv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
)

func TestPrivateKey(t *testing.T) {
	tests := []struct {
		name          string
		spec          *accessgraphv1pb.PrivateKeySpec
		errValidation require.ErrorAssertionFunc
	}{
		{
			name: "valid derived",
			spec: &accessgraphv1pb.PrivateKeySpec{
				DeviceId:             uuid.New().String(),
				PublicKeyFingerprint: "fingerprint",
				PublicKeyMode:        accessgraphv1pb.PublicKeyMode_PUBLIC_KEY_MODE_DERIVED,
			},
			errValidation: require.NoError,
		},
		{
			name: "valid file",
			spec: &accessgraphv1pb.PrivateKeySpec{
				DeviceId:             uuid.New().String(),
				PublicKeyFingerprint: "fingerprint",
				PublicKeyMode:        accessgraphv1pb.PublicKeyMode_PUBLIC_KEY_MODE_PUB_FILE,
			},
			errValidation: require.NoError,
		},
		{
			name: "missing fingerprint derived",
			spec: &accessgraphv1pb.PrivateKeySpec{
				DeviceId:             uuid.New().String(),
				PublicKeyFingerprint: "",
				PublicKeyMode:        accessgraphv1pb.PublicKeyMode_PUBLIC_KEY_MODE_DERIVED,
			},
			errValidation: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "PublicKeyFingerprint is unset")
			},
		},
		{
			name: "missing fingerprint file",
			spec: &accessgraphv1pb.PrivateKeySpec{
				DeviceId:             uuid.New().String(),
				PublicKeyFingerprint: "",
				PublicKeyMode:        accessgraphv1pb.PublicKeyMode_PUBLIC_KEY_MODE_PUB_FILE,
			},
			errValidation: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "PublicKeyFingerprint is unset")
			},
		},
		{
			name: "valid protected",
			spec: &accessgraphv1pb.PrivateKeySpec{
				DeviceId:             uuid.New().String(),
				PublicKeyFingerprint: "", /* empty fingerprint */
				PublicKeyMode:        accessgraphv1pb.PublicKeyMode_PUBLIC_KEY_MODE_PROTECTED,
			},
			errValidation: require.NoError,
		},
		{
			name: "invalid public key ode",
			spec: &accessgraphv1pb.PrivateKeySpec{
				DeviceId:             uuid.New().String(),
				PublicKeyFingerprint: "fingerprint",
				PublicKeyMode:        500,
			},
			errValidation: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "PublicKeyMode is invalid")
			},
		},
		{
			name: "missing DeviceId",
			spec: &accessgraphv1pb.PrivateKeySpec{
				PublicKeyFingerprint: "fingerprint",
				PublicKeyMode:        accessgraphv1pb.PublicKeyMode_PUBLIC_KEY_MODE_PROTECTED,
			},
			errValidation: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "DeviceId is unset")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			privKey, err := NewPrivateKey(tt.spec)
			tt.errValidation(t, err)
			if err != nil {
				return
			}
			require.NotEmpty(t, privKey.Metadata.Name)
			require.Empty(t, cmp.Diff(tt.spec, privKey.Spec, protocmp.Transform()))

		})
	}
}
