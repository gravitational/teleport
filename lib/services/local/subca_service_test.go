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
	"crypto/x509"
	"slices"
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

	const caType = types.WindowsCA

	// sharedEnv is shared by failure tests, since they aren't expected to change
	// the underlying resource.
	sharedEnv := subcaenv.New(t, subcaenv.EnvParams{
		CATypesToCreate:  []types.CertAuthType{caType},
		SkipExternalRoot: true,
	})

	// External CA chain.
	const chainLength = 3
	caChain := sharedEnv.MakeCAChain(t, chainLength)
	leafToRootChain := caChain.LeafToRootPEMs()
	// Create overrides from the tip of the external chain.
	sharedEnv.ExternalRoot = caChain[len(caChain)-1]

	cloneEnv := func(t *testing.T) *subcaenv.Env {
		env := subcaenv.New(t, subcaenv.EnvParams{
			CATypesToCreate:  []types.CertAuthType{caType},
			SkipExternalRoot: true,
		})
		env.ExternalRoot = sharedEnv.ExternalRoot
		return env
	}

	// Cloned by failure tests.
	sharedCAOverride := sharedEnv.NewOverrideForCAType(t, caType)

	// Used to test various public key mismatch scenarios. "Random".
	const unrelatedPublicKey = `9852b3bbc867cc047e6d894333488da322df27fa96aa20ebb29c0bf44ff6327f`

	// Forge a CA that has the correct Subject to match the override certificate,
	// but has a different set of keys.
	forgedExternalRoot, err := subcaenv.NewSelfSignedCA(&subcaenv.CAParams{
		Clock: sharedEnv.Clock,
		Template: &x509.Certificate{
			Subject: sharedEnv.ExternalRoot.Cert.Subject,
		},
	})
	require.NoError(t, err)

	// Verify that resources are written under the correct customized key.
	t.Run("storage key", func(t *testing.T) {
		t.Parallel()

		env := cloneEnv(t)
		be := env.Backend
		ctx := t.Context()

		// Create resource.
		caOverride := env.NewOverrideForCAType(t, caType)
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
		success bool // mutually exclusive with wantErr
		wantErr string
	}{
		{
			name: "OK: Valid CA override",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				// Don't modify anything, take the default testenv override.
			},
			success: true,
		},
		{
			name: "OK: Minimal CA override",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.Spec = &subcav1.CertAuthorityOverrideSpec{}
			},
			success: true,
		},

		{
			name: "empty kind",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.Kind = ""
			},
			wantErr: "kind",
		},
		{
			name: "invalid kind",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.Kind = types.KindCertAuthority // wrong type
			},
			wantErr: "kind",
		},
		{
			name: "empty sub_kind",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.SubKind = ""
			},
			wantErr: "sub_kind",
		},
		{
			name: "invalid sub_kind",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.SubKind = string(types.DatabaseCA) // not allowed
			},
			wantErr: "sub_kind",
		},
		{
			name: "empty version",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.Version = ""
			},
			wantErr: "version",
		},
		{
			name: "invalid version",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.Version = types.V2
			},
			wantErr: "version",
		},
		{
			name: "nil metadata",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.Metadata = nil
			},
			wantErr: "name/clusterName required",
		},
		{
			name: "empty name",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.Metadata.Name = ""
			},
			wantErr: "name/clusterName required",
		},
		{
			name: "nil spec",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.Spec = nil
			},
			wantErr: "spec required",
		},
		{
			name: "nil certificate_override",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.Spec.CertificateOverrides = append(ca.Spec.CertificateOverrides, nil)
			},
			wantErr: "nil certificate override",
		},
		{
			name: "certificate_override: empty certificate and public key (enabled)",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.Spec.CertificateOverrides[0].Certificate = ""
				ca.Spec.CertificateOverrides[0].PublicKey = ""
				ca.Spec.CertificateOverrides[0].Disabled = false
			},
			wantErr: "certificate required",
		},
		{
			name: "certificate_override: empty certificate and public key (disabled)",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.Spec.CertificateOverrides[0].Certificate = ""
				ca.Spec.CertificateOverrides[0].PublicKey = ""
				ca.Spec.CertificateOverrides[0].Disabled = true
			},
			wantErr: "certificate or public key required",
		},
		{
			name: "certificate_override: invalid certificate",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.Spec.CertificateOverrides[0].Certificate = "ceci n'est pas a certificate"
			},
			wantErr: "expected PEM",
		},
		{
			name: "certificate_override: invalid public key",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.Spec.CertificateOverrides[0].PublicKey = "not a valid key"
			},
			wantErr: "invalid public key",
		},
		{
			name: "certificate_override: certificate and public key mismatch",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				// Doesn't match the Certificate field.
				ca.Spec.CertificateOverrides[0].PublicKey = unrelatedPublicKey
			},
			wantErr: "public key mismatch",
		},
		{
			name: "certificate_override: chain without certificate",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				co.Chain = leafToRootChain
				co.Certificate = ""
				co.Disabled = true
			},
			wantErr: "chain not allowed with an empty certificate",
		},
		{
			name: "certificate_override: chain certificate invalid",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				co.PublicKey = ""
				co.Chain = []string{
					leafToRootChain[0],
					"ceci n'est pas a certificate",
					leafToRootChain[1],
					leafToRootChain[2],
				}
			},
			wantErr: "chain[1]: expected PEM",
		},
		{
			name: "certificate_override: certificate included in chain",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				co.PublicKey = ""
				co.Chain = append([]string{co.Certificate}, leafToRootChain...)
			},
			wantErr: "override certificate should not be included",
		},
		{
			name: "certificate_override: chain out of order",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				co.PublicKey = ""
				co.Chain = caChain.RootToLeafPEMs() // reverse order
			},
			wantErr: "chain out of order",
		},
		{
			name: "certificate_override: chain signature invalid (forged CA)",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				co.Chain = append(
					[]string{string(forgedExternalRoot.CertPEM)},
					leafToRootChain[1:]...,
				)
			},
			wantErr: "chain signature check failed",
		},
		{
			name: "certificate_override: chain has too many entries",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				co.PublicKey = ""
				co.Chain = slices.Repeat([]string{leafToRootChain[0]}, 20)
			},
			wantErr: "chain has too many entries",
		},
		{
			name: "certificate_override: duplicate public key",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				ca.Spec.CertificateOverrides = append(
					ca.Spec.CertificateOverrides,
					&subcav1.CertificateOverride{PublicKey: co.PublicKey, Disabled: true},
				)
			},
			wantErr: "duplicate override",
		},

		{
			name: "OK: Enabled override",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				co.Disabled = false
			},
			success: true,
		},
		{
			name: "OK: Override without public key",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				co.PublicKey = ""
			},
			success: true,
		},
		{
			name: "OK: Disabled override with only public key",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				co.Certificate = ""
			},
			success: true,
		},
		{
			name: "OK: Override with chain",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				co.Chain = leafToRootChain
			},
			success: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Create a distinct env for success tests, but otherwise use shared
			// resources.
			env := sharedEnv
			var caOverride *subcav1.CertAuthorityOverride
			if test.success {
				env = cloneEnv(t)
				caOverride = env.NewOverrideForCAType(t, caType)
			} else {
				caOverride = proto.Clone(sharedCAOverride).(*subcav1.CertAuthorityOverride)
			}
			service := env.SubCA

			test.modify(caOverride)

			// Take a copy. generic.Service modifies its inputs.
			want := proto.Clone(caOverride).(*subcav1.CertAuthorityOverride)

			got, err := service.CreateCertAuthorityOverride(t.Context(), caOverride)
			if !test.success {
				// Assert failures.
				if assert.ErrorContains(t, err, test.wantErr, "Create error mismatch") {
					assert.ErrorAs(t, err, new(*trace.BadParameterError), "Create error type mismatch")
				}
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
