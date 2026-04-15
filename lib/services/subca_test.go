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

package services_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func TestMarshalCertAuthOverrideRoundtrip(t *testing.T) {
	want := &subcav1.CertAuthorityOverride{
		Kind:    types.KindCertAuthorityOverride,
		SubKind: string(types.DatabaseClientCA),
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "zarquon",
		},
		Spec: &subcav1.CertAuthorityOverrideSpec{},
	}

	t.Run("regular", func(t *testing.T) {
		val, err := services.MarshalCertAuthorityOverride(want)
		require.NoError(t, err)

		got, err := services.UnmarshalCertAuthorityOverride(val)
		require.NoError(t, err)
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("CAOverride mismatch (-want +got)\n%s", diff)
		}
	})

	t.Run("dynamic", func(t *testing.T) {
		val, err := services.MarshalResource(types.Resource153ToLegacy(want))
		require.NoError(t, err)

		wrapped, err := services.UnmarshalResource(types.KindCertAuthorityOverride, val)
		require.NoError(t, err)
		unwrapper, ok := wrapped.(types.Resource153UnwrapperT[*subcav1.CertAuthorityOverride])
		require.True(t, ok, "Wrapped resource has unexpected type: %T", wrapped)
		got := unwrapper.UnwrapT()
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("CAOverride mismatch (-want +got)\n%s", diff)
		}
	})
}
