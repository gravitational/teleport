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

package local_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services/local"
	subcaenv "github.com/gravitational/teleport/lib/subca/testenv"
)

func TestSubCAService_Create(t *testing.T) {
	t.Parallel()

	const caType1 = types.DatabaseClientCA // for test table
	const caType2 = types.WindowsCA        // for "storage key" test

	env := subcaenv.New(t, subcaenv.EnvParams{
		CATypesToCreate: []types.CertAuthType{
			caType1,
			caType2,
		},
	})
	service := env.SubCA

	// Cloned before every test.
	sharedCAOverride := env.NewOverrideForCAType(t, caType1)

	t.Run("nil resource", func(t *testing.T) {
		t.Parallel()
		_, err := service.CreateCertAuthorityOverride(t.Context(), nil)
		assert.ErrorContains(t, err, "name/clusterName required", "Create error mismatch")
	})

	// Verify that resources are written under the correct customized key.
	t.Run("storage key", func(t *testing.T) {
		t.Parallel()

		be := env.Backend
		ctx := t.Context()

		// Create resource. Uses a different caType from sharedCAOverride to not
		// interfere in the test table.
		caOverride := env.NewOverrideForCAType(t, caType2)
		_, err := env.SubCA.CreateCertAuthorityOverride(ctx, caOverride)
		require.NoError(t, err, "CreateCertAuthorityOverride errored")

		// Form our customized key.
		wantKey := backend.NewKey(
			"cert_authority_overrides",
			"cluster",
			caOverride.Metadata.Name,
			caOverride.SubKind,
		)

		// Get resource from customized key.
		_, err = be.Get(ctx, wantKey)
		require.NoError(t, err, "Read resource by customized key")

		// Verify that the "normal" generic.Service key doesn't exist.
		notWantKey := backend.NewKey(
			"cert_authority_overrides",
			"cluster",
			caOverride.Metadata.Name,
		)
		_, err = be.Get(ctx, notWantKey)
		assert.ErrorAs(t, err, new(*trace.NotFoundError), "Read resource by notWantKey")
	})

	tests := []struct {
		name    string
		modify  func(ca *subcav1.CertAuthorityOverride)
		wantErr string
	}{
		{
			name: "OK: Valid CA override",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				// Don't modify anything, take the default testenv override.
			},
		},
		{
			name: "CAOverride is validated",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.Spec.CertificateOverrides[0].Certificate = "ceci n'est pas a certificate"
			},
			wantErr: "expected PEM",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			caOverride := proto.Clone(sharedCAOverride).(*subcav1.CertAuthorityOverride)
			test.modify(caOverride)

			// Take a copy. generic.Service modifies its inputs.
			want := proto.Clone(caOverride).(*subcav1.CertAuthorityOverride)

			got, err := service.CreateCertAuthorityOverride(t.Context(), caOverride)
			if test.wantErr != "" {
				// Assert failures.
				require.ErrorContains(t, err, test.wantErr, "Create error mismatch")
				assert.ErrorAs(t, err, new(*trace.BadParameterError), "Create error type mismatch")
				return
			}
			// Assert success.
			require.NoError(t, err, "CreateCertAuthorityOverride errored")
			want.Metadata.Revision = got.Metadata.Revision
			if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
				t.Errorf("Create mismatch (-want +got)\n%s", diff)
			}

			// Assert stored resource.
			stored, err := service.GetCertAuthorityOverride(t.Context(), local.CertAuthorityOverrideIDFromResource(got))
			require.NoError(t, err, "GetCertAuthorityOverride errored")
			if diff := cmp.Diff(got, stored, protocmp.Transform()); diff != "" {
				t.Errorf("Get mismatch (-want +got)\n%s", diff)
			}
		})
	}
}
