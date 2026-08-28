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

func TestAuthorizedKey(t *testing.T) {
	tests := []struct {
		name          string
		spec          *accessgraphv1pb.AuthorizedKeySpec
		errValidation require.ErrorAssertionFunc
	}{
		{
			name: "valid",
			spec: &accessgraphv1pb.AuthorizedKeySpec{
				HostId:         uuid.New().String(),
				KeyFingerprint: "fingerprint",
				HostUser:       "user",
				KeyType:        "ssh-rsa",
			},
			errValidation: require.NoError,
		},
		{
			name: "missing fingerprint",
			spec: &accessgraphv1pb.AuthorizedKeySpec{
				HostId:         uuid.New().String(),
				KeyFingerprint: "",
				HostUser:       "user",
				KeyType:        "ssh-rsa",
			},
			errValidation: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "KeyFingerprint is unset")
			},
		},
		{
			name: "missing user",
			spec: &accessgraphv1pb.AuthorizedKeySpec{
				HostId:         uuid.New().String(),
				KeyFingerprint: "fingerprint",
				HostUser:       "",
				KeyType:        "ssh-rsa",
			},
			errValidation: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "HostUser is unset")
			},
		},
		{
			name: "missing HostID",
			spec: &accessgraphv1pb.AuthorizedKeySpec{
				KeyFingerprint: "fingerprint",
				HostUser:       "user",
				KeyType:        "ssh-rsa",
			},
			errValidation: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "HostId is unset")
			},
		},
		{
			name: "missing HostID",
			spec: &accessgraphv1pb.AuthorizedKeySpec{
				KeyFingerprint: "fingerprint",
				HostUser:       "user",
				HostId:         uuid.New().String(),
			},
			errValidation: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "KeyType is unset")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			privKey, err := NewAuthorizedKey(tt.spec)
			tt.errValidation(t, err)
			if err != nil {
				return
			}
			require.NotEmpty(t, privKey.Metadata.Name)
			require.Empty(t, cmp.Diff(tt.spec, privKey.Spec, protocmp.Transform()))

		})
	}
}
