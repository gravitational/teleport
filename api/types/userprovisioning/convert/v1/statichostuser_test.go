// Copyright 2024 Gravitational, Inc.
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

package v1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/userprovisioning"
)

func TestRoundtrip(t *testing.T) {
	t.Parallel()
	hostUser := newStaticHostUser()
	converted, err := FromProto(ToProto(hostUser))
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(hostUser, converted,
		cmpopts.IgnoreUnexported(headerv1.ResourceHeader{}, headerv1.Metadata{})))
}

func TestNoPanicOnNilSpec(t *testing.T) {
	hostUser := ToProto(newStaticHostUser())
	hostUser.Spec = nil
	_, err := FromProto(hostUser)
	require.Error(t, err)
}

func newStaticHostUser() *userprovisioning.StaticHostUser {
	return userprovisioning.NewStaticHostUser(&headerv1.Metadata{
		Name: "test-user",
	}, userprovisioning.Spec{
		Login:   "alice",
		Groups:  []string{"foo", "bar"},
		Sudoers: []string{"abcd1234"},
		Uid:     "1234",
		Gid:     "5678",
		NodeLabels: types.Labels{
			"foo": {"bar"},
		},
		NodeLabelsExpression: "labels['foo'] == labels['bar']",
	})
}
