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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services/local"
	subcaenv "github.com/gravitational/teleport/lib/subca/testenv"
)

// TestSubCAService_CRUD tests the CRUD operations of SubCAService, with the
// exclusion of Create which is extensively tested elsewhere.
//
// Most corner-cases and validation scenarios are covered by
// TestSubCAService_Create, this focus more on the happy path.
func TestSubCAService_CRUD(t *testing.T) {
	t.Parallel()

	assertStored := func(
		t *testing.T,
		service *local.SubCAService,
		want *subcav1.CertAuthorityOverride,
	) {
		t.Helper()

		id := local.CertAuthorityOverrideIDFromResource(want)
		got, err := service.GetCertAuthorityOverride(t.Context(), id)
		require.NoError(t, err)
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("Stored CA override mismatch (-want +got)\n%s", diff)
		}
	}

	t.Run("Upsert (create)", func(t *testing.T) {
		t.Parallel()

		const caType = types.WindowsCA
		env := subcaenv.New(t, subcaenv.EnvParams{
			CATypesToCreate: []types.CertAuthType{caType},
		})
		service := env.SubCA

		caOverride := env.NewOverrideForCAType(t, caType)
		want := proto.Clone(caOverride).(*subcav1.CertAuthorityOverride)

		// Upsert and verify response.
		created, err := service.UpsertCertAuthorityOverride(t.Context(), caOverride)
		require.NoError(t, err, "UpsertCertAuthorityOverride errored")
		want.GetMetadata().SetRevision(created.GetMetadata().GetRevision())
		if diff := cmp.Diff(want, created, protocmp.Transform()); diff != "" {
			t.Errorf("Upsert mismatch (-want +got)\n%s", diff)
		}

		// Assert against storage.
		assertStored(t, service, want)

		invalid := proto.Clone(created).(*subcav1.CertAuthorityOverride)
		invalid.GetSpec().GetCertificateOverrides()[0].SetCertificate("ceci n'est pas a certificate")
		const wantInvalidErr = "certificate: expected PEM"

		t.Run("Upsert (invalid)", func(t *testing.T) {
			_, err := service.UpsertCertAuthorityOverride(t.Context(), invalid)
			assert.ErrorContains(t, err, wantInvalidErr, "UpsertCertAuthorityOverride error mismatch")
		})

		t.Run("Update (invalid)", func(t *testing.T) {
			_, err := service.UpdateCertAuthorityOverride(t.Context(), invalid)
			assert.ErrorContains(t, err, wantInvalidErr, "UpdateCertAuthorityOverride error mismatch")
		})

		t.Run("Upsert (update)", func(t *testing.T) {
			co := created.GetSpec().GetCertificateOverrides()[0]
			co.SetDisabled(false) // Enable override.

			want := proto.Clone(created).(*subcav1.CertAuthorityOverride)

			updated, err := service.UpsertCertAuthorityOverride(t.Context(), created)
			require.NoError(t, err, "UpsertCertAuthorityOverride errored")
			assert.NotEqual(t,
				want.GetMetadata().GetRevision(), updated.GetMetadata().GetRevision(),
				"Revision unchanged after update")
			want.GetMetadata().SetRevision(updated.GetMetadata().GetRevision())
			if diff := cmp.Diff(want, updated, protocmp.Transform()); diff != "" {
				t.Errorf("Upsert mismatch (-want +got)\n%s", diff)
			}

			assertStored(t, service, updated)
		})

		t.Run("Update", func(t *testing.T) {
			ctx := t.Context()

			// Create an altogether different override.
			caOverride2 := env.NewOverrideForCAType(t, caType)
			// Enable, like the previous update.
			caOverride2.GetSpec().GetCertificateOverrides()[0].SetDisabled(false)

			// Fetch stored override...
			id := local.CertAuthorityOverrideIDFromResource(created)
			stored, err := service.GetCertAuthorityOverride(ctx, id)
			require.NoError(t, err, "GetCertAuthorityOverride errored")

			// ...and replace its contents with "caOverride2".
			in := stored
			in.GetSpec().SetCertificateOverrides(caOverride2.GetSpec().GetCertificateOverrides())

			want := proto.Clone(in).(*subcav1.CertAuthorityOverride)

			updated, err := service.UpdateCertAuthorityOverride(ctx, in)
			require.NoError(t, err, "UpdateCertAuthorityOverride errored")
			assert.NotEqual(t,
				want.GetMetadata().GetRevision(), updated.GetMetadata().GetRevision(),
				"Revision unchanged after update")
			want.GetMetadata().SetRevision(updated.GetMetadata().GetRevision())
			if diff := cmp.Diff(want, updated, protocmp.Transform()); diff != "" {
				t.Errorf("Upsert mismatch (-want +got)\n%s", diff)
			}

			assertStored(t, service, updated)
		})

		t.Run("Delete", func(t *testing.T) {
			ctx := t.Context()
			id := local.CertAuthorityOverrideIDFromResource(created)

			// 1st delete succeeds.
			require.NoError(t,
				service.DeleteCertAuthorityOverride(ctx, id),
				"DeleteCertAuthorityOverride errored")

			// 2nd delete errors with NotFound.
			assert.ErrorAs(t,
				service.DeleteCertAuthorityOverride(ctx, id),
				new(*trace.NotFoundError),
				"DeleteCertAuthorityOverride error mismatch")

			// Get also NotFounds.
			_, err := service.GetCertAuthorityOverride(ctx, id)
			assert.ErrorAs(t,
				err,
				new(*trace.NotFoundError),
				"GetCertAuthorityOverride error mismatch")
		})
	})

	t.Run("List", func(t *testing.T) {
		t.Parallel()

		const caType1 = types.DatabaseClientCA
		const caType2 = types.WindowsCA
		env := subcaenv.New(t, subcaenv.EnvParams{
			CATypesToCreate: []types.CertAuthType{
				caType1,
				caType2,
			},
		})

		service := env.SubCA
		ctx := t.Context()

		caOverride1, err := service.UpsertCertAuthorityOverride(ctx, env.NewOverrideForCAType(t, caType1))
		require.NoError(t, err)
		caOverride2, err := service.UpsertCertAuthorityOverride(ctx, env.NewOverrideForCAType(t, caType2))
		require.NoError(t, err)

		const pageSize = 0 // use defaults
		pageToken := ""
		resp, nextPageToken, err := service.ListCertAuthorityOverrides(ctx, pageSize, pageToken)
		require.NoError(t, err, "ListCertAuthorityOverrides errored")
		assert.Empty(t, nextPageToken, "nextPageToken is not empty, this is unexpected")

		want := []*subcav1.CertAuthorityOverride{
			caOverride1, // Note: already in key/list order.
			caOverride2,
		}
		if diff := cmp.Diff(want, resp, protocmp.Transform()); diff != "" {
			t.Errorf("List mismatch (-want +got)\n%s", diff)
		}
	})
}

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
			caOverride.GetMetadata().GetName(),
			caOverride.GetSubKind(),
		)

		// Get resource from customized key.
		_, err = be.Get(ctx, wantKey)
		require.NoError(t, err, "Read resource by customized key")

		// Verify that the "normal" generic.Service key doesn't exist.
		notWantKey := backend.NewKey(
			"cert_authority_overrides",
			"cluster",
			caOverride.GetMetadata().GetName(),
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
				ca.GetSpec().GetCertificateOverrides()[0].SetCertificate("ceci n'est pas a certificate")
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
			want.GetMetadata().SetRevision(got.GetMetadata().GetRevision())
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

func TestSubCAService_Update_wrongRevision(t *testing.T) {
	t.Parallel()

	const caType = types.WindowsCA
	env := subcaenv.New(t, subcaenv.EnvParams{
		CATypesToCreate: []types.CertAuthType{caType},
	})
	service := env.SubCA
	ctx := t.Context()

	caOverride := env.NewOverrideForCAType(t, caType)
	caOverride, err := service.CreateCertAuthorityOverride(ctx, caOverride)
	require.NoError(t, err)
	rev1 := caOverride.GetMetadata().GetRevision()

	caOverride, err = service.UpdateCertAuthorityOverride(ctx, caOverride)
	require.NoError(t, err)
	rev2 := caOverride.GetMetadata().GetRevision()
	require.NotEqual(t, rev1, rev2, "Revision didn't change on update")

	// Update using an old revision.
	caOverride.GetMetadata().SetRevision(rev1)
	_, err = service.UpdateCertAuthorityOverride(ctx, caOverride)
	assert.ErrorAs(t, err, new(*trace.CompareFailedError),
		"UpdateCertAuthorityOverride() revision mismatch error")
}

func TestSubCAService_GetDeleteNotFoundError(t *testing.T) {
	t.Parallel()

	env := subcaenv.New(t, subcaenv.EnvParams{
		SkipExternalRoot: true,
	})
	service := env.SubCA

	id := local.CertAuthorityOverrideID{
		ClusterName: env.ClusterName,
		CAType:      string(types.WindowsCA),
	}
	wantErr := fmt.Sprintf(`"%s/%s" doesn't exist`, id.CAType, id.ClusterName)

	t.Run("Get", func(t *testing.T) {
		t.Parallel()
		_, err := service.GetCertAuthorityOverride(t.Context(), id)
		assert.ErrorContains(t, err, wantErr)
	})

	t.Run("Delete", func(t *testing.T) {
		t.Parallel()
		err := service.DeleteCertAuthorityOverride(t.Context(), id)
		assert.ErrorContains(t, err, wantErr)
	})
}

func TestSubCAService_ConditionalDeleteCertAuthorityOverride(t *testing.T) {
	t.Parallel()

	const caType = types.DatabaseClientCA
	const caTypeOther = types.WindowsCA
	env := subcaenv.New(t, subcaenv.EnvParams{
		CATypesToCreate: []types.CertAuthType{caType},
	})
	service := env.SubCA

	// Create CA override "rev1".
	caOverride := env.NewOverrideForCAType(t, caType)
	caOverride.GetMetadata().SetDescription("rev1")
	caOverride.GetSpec().SetCertificateOverrides(nil) // not needed for this test
	caOverride1, err := service.CreateCertAuthorityOverride(t.Context(), caOverride)
	require.NoError(t, err)
	rev1 := caOverride1.GetMetadata().GetRevision()

	// "rev2" (current).
	caOverride.GetMetadata().SetDescription("rev2")
	caOverride.GetMetadata().SetRevision(rev1)
	caOverride2, err := service.UpdateCertAuthorityOverride(t.Context(), caOverride)
	require.NoError(t, err)
	rev2 := caOverride2.GetMetadata().GetRevision()

	validID := local.CertAuthorityOverrideIDFromResource(caOverride)

	tests := []struct {
		name        string
		id          local.CertAuthorityOverrideID
		revision    string
		wantErr     string
		wantErrType any
	}{
		{
			name:     "id empty",
			revision: rev2,
			wantErr:  "clusterName required",
		},
		{
			name: "id.ClusterName empty",
			id: func() local.CertAuthorityOverrideID {
				id := validID
				id.ClusterName = ""
				return id
			}(),
			revision: rev2,
			wantErr:  "clusterName required",
		},
		{
			name: "id.CAType empty",
			id: func() local.CertAuthorityOverrideID {
				id := validID
				id.CAType = ""
				return id
			}(),
			revision: rev2,
			wantErr:  "caType required",
		},
		{
			name: "CA override not found",
			id: func() local.CertAuthorityOverrideID {
				id := validID
				id.CAType = string(caTypeOther)
				return id
			}(),
			revision:    rev2,
			wantErr:     "does not exist or revision does not match",
			wantErrType: new(*trace.CompareFailedError),
		},
		{
			name:    "revision empty",
			id:      validID,
			wantErr: "revision required",
		},
		{
			name:        "revision not found",
			id:          validID,
			revision:    rev1, // current revision is rev2
			wantErr:     "does not exist or revision does not match",
			wantErrType: new(*trace.CompareFailedError),
		},
		{
			name:     "ok",
			id:       validID,
			revision: rev2,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := service.ConditionalDeleteCertAuthorityOverride(t.Context(), test.id, test.revision)
			if test.wantErr != "" {
				assert.ErrorContains(t, err, test.wantErr, "ConditionalDeleteCertAuthorityOverride error mismatch")
			}
			if test.wantErrType != nil {
				assert.ErrorAs(t, err, test.wantErrType, "ConditionalDeleteCertAuthorityOverride error type mismatch")
			}
			if test.wantErr != "" || test.wantErrType != nil {
				return // Asserted above.
			}
			require.NoError(t, err)

			// Verify successful deletion.
			_, err = service.GetCertAuthorityOverride(t.Context(), test.id)
			assert.ErrorAs(t, err, new(*trace.NotFoundError), "Get error mismatch after ConditionalDelete")
		})
	}
}

func TestCreateResource_CertAuthorityOverride(t *testing.T) {
	t.Parallel()

	env := subcaenv.New(t, subcaenv.EnvParams{
		SkipExternalRoot: true,
	})
	be := env.Backend
	service := env.SubCA

	t.Run("invalid", func(t *testing.T) {
		t.Parallel()

		// An empty resource is not valid.
		r := types.Resource153ToLegacy(&subcav1.CertAuthorityOverride{})

		assert.ErrorAs(t,
			local.CreateResources(t.Context(), be, r),
			new(*trace.BadParameterError),
			"CreateResources error mismatch")
	})

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		want := subcav1.CertAuthorityOverride_builder{
			Kind:    types.KindCertAuthorityOverride,
			SubKind: string(types.DatabaseClientCA),
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name: env.ClusterName,
			}.Build(),
			Spec: &subcav1.CertAuthorityOverrideSpec{},
		}.Build()

		// CreateResources.
		r := types.Resource153ToLegacy(want)
		require.NoError(t,
			local.CreateResources(t.Context(), be, r),
			"CreateResources errored")

		// Verify resource via service read.
		got, err := service.GetCertAuthorityOverride(
			t.Context(), local.CertAuthorityOverrideIDFromResource(want))
		require.NoError(t, err)
		want.GetMetadata().SetRevision(got.GetMetadata().GetRevision())
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("CertAuthorityOverride mismatch (-want +got)\n%s", diff)
		}
	})
}

func TestSubCAService_PendingCSRRequest_CRUD(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	env := subcaenv.New(t, subcaenv.EnvParams{
		Clock: clock,
	})
	service := env.SubCA

	const pkh1 = "ea16c3a8c1f31943019ecc9bfb2899b60e8ec156874bdf4606a899c95392cef3"
	const pkh2 = "1cd6a96e049f643d1f8c1cdd0390c08c2a7587df204ba254ee46009c08e80456"
	req := subcav1.PendingCSRRequest_builder{
		Kind:    types.KindPendingCSRRequest,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: uuid.NewString(),
		}.Build(),
		Spec: subcav1.PendingCSRRequestSpec_builder{
			ClusterName: env.ClusterName,
			CaType:      string(types.WindowsCA),
			PublicKeyHashes: []*subcav1.PublicKeyHash{
				subcav1.PublicKeyHash_builder{Value: pkh1}.Build(),
				subcav1.PublicKeyHash_builder{Value: pkh2}.Build(),
			},
		}.Build(),
	}.Build()

	if !t.Run("create", func(t *testing.T) {
		got, err := service.CreatePendingCSRRequest(t.Context(), proto.CloneOf(req))
		require.NoError(t, err)
		// Drift clock so we can detect eventual changes to metadata.expires.
		clock.Advance(1 * time.Minute)

		assert.NotNil(t, got.GetMetadata().GetExpires(), "PendingCSRRequest.Metadata.Expires is nil")

		req.GetMetadata().SetExpires(got.GetMetadata().GetExpires())
		req.GetMetadata().SetRevision(got.GetMetadata().GetRevision())
		if diff := cmp.Diff(req, got, protocmp.Transform()); diff != "" {
			t.Errorf("Create mismatch (-want +got)\n%s", diff)
		}
	}) {
		t.Skip("create test failed, skipping other tests")
	}

	name := req.GetMetadata().GetName()

	t.Run("update", func(t *testing.T) {
		const csr = `
-----BEGIN CERTIFICATE REQUEST-----
CSR GOES HERE
-----END CERTIFICATE REQUEST-----`

		newReq := proto.CloneOf(req)
		newReq.SetStatus(subcav1.PendingCSRRequestStatus_builder{
			PublicKeyHashToPendingCsr: map[string]*subcav1.PendingCSR{
				pkh1: subcav1.PendingCSR_builder{
					Status: &status.Status{}, // zero == success
					Csr: subcav1.CertificateSigningRequest_builder{
						Pem: strings.TrimSpace(csr),
					}.Build(),
				}.Build(),
				pkh2: subcav1.PendingCSR_builder{
					Status: &status.Status{
						Code:    int32(codes.Internal),
						Message: "failed to sign CSR",
					},
				}.Build(),
			},
		}.Build())

		updated, err := service.UpdatePendingCSRRequest(t.Context(), proto.CloneOf(newReq))
		require.NoError(t, err)
		// Assign updated so other tests use the latest instance.
		req = updated

		newReq.GetMetadata().SetRevision(updated.GetMetadata().GetRevision())
		if diff := cmp.Diff(newReq, updated, protocmp.Transform()); diff != "" {
			t.Errorf("Update mismatch (-want +got)\n%s", diff)
		}
	})

	t.Run("get", func(t *testing.T) {
		got, err := service.GetPendingCSRRequest(t.Context(), name)
		require.NoError(t, err)

		if diff := cmp.Diff(req, got, protocmp.Transform()); diff != "" {
			t.Errorf("Get mismatch (-want +got)\n%s", diff)
		}
	})

	t.Run("list", func(t *testing.T) {
		const pageSize = 0
		const pageToken = ""
		got, nextPageToken, err := service.ListPendingCSRRequest(t.Context(), pageSize, pageToken)
		require.NoError(t, err)
		assert.Empty(t, nextPageToken, "nextPageToken not empty")

		want := []*subcav1.PendingCSRRequest{req}
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("List mismatch (-want +got)\n%s", diff)
		}
	})

	t.Run("delete", func(t *testing.T) {
		require.NoError(t, service.DeletePendingCSRRequest(t.Context(), name))

		assert.ErrorAs(t,
			service.DeletePendingCSRRequest(t.Context(), name),
			new(*trace.NotFoundError),
		)
	})
}

func TestSubCAService_PendingCSRRequest_validation(t *testing.T) {
	t.Parallel()

	env := subcaenv.New(t, subcaenv.EnvParams{})
	service := env.SubCA

	const pkh1 = "ea16c3a8c1f31943019ecc9bfb2899b60e8ec156874bdf4606a899c95392cef3"
	const pkh2 = "1cd6a96e049f643d1f8c1cdd0390c08c2a7587df204ba254ee46009c08e80456"
	const pkhUnrelated = "4531b8b283404ec913dc02c136efa16aea35465d19ec828b52056a1b593ecac1"

	makeReq := func(modify func(*subcav1.PendingCSRRequest)) *subcav1.PendingCSRRequest {
		validReq := subcav1.PendingCSRRequest_builder{
			Kind:    types.KindPendingCSRRequest,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name: "aa8fa139-c4bf-4cb8-821a-3be57565d231",
			}.Build(),
			Spec: subcav1.PendingCSRRequestSpec_builder{
				ClusterName: env.ClusterName,
				CaType:      string(types.DatabaseClientCA),
				PublicKeyHashes: []*subcav1.PublicKeyHash{
					subcav1.PublicKeyHash_builder{Value: pkh1}.Build(),
					subcav1.PublicKeyHash_builder{Value: pkh2}.Build(),
				},
			}.Build(),
		}.Build()
		modify(validReq)
		return validReq
	}

	tests := []struct {
		name    string
		req     *subcav1.PendingCSRRequest
		wantErr string
	}{
		{
			name: "cluster_name required",
			req: makeReq(func(req *subcav1.PendingCSRRequest) {
				req.GetSpec().SetClusterName("")
			}),
			wantErr: "spec.cluster_name",
		},
		{
			name: "ca_type required",
			req: makeReq(func(req *subcav1.PendingCSRRequest) {
				req.GetSpec().SetCaType("")
			}),
			wantErr: "spec.ca_type",
		},
		{
			name: "custom_subject invalid",
			req: makeReq(func(req *subcav1.PendingCSRRequest) {
				req.GetSpec().SetCustomSubject(subcav1.DistinguishedName_builder{
					Names: []*subcav1.AttributeTypeAndValue{
						nil, // invalid
					},
				}.Build())
			}),
			wantErr: "custom_subject",
		},
		{
			name: "public_key_hashes required",
			req: makeReq(func(req *subcav1.PendingCSRRequest) {
				req.GetSpec().SetPublicKeyHashes([]*subcav1.PublicKeyHash{})
			}),
			wantErr: "public_key_hashes required",
		},
		{
			name: "public_key_hashes must not be empty",
			req: makeReq(func(req *subcav1.PendingCSRRequest) {
				pkhs := req.GetSpec().GetPublicKeyHashes()
				pkhs = append(pkhs, nil)
				req.GetSpec().SetPublicKeyHashes(pkhs)
			}),
			wantErr: "public_key_hashes[2]: value required",
		},
		{
			name: "status CSR keys must match spec keys",
			req: makeReq(func(req *subcav1.PendingCSRRequest) {
				req.SetStatus(subcav1.PendingCSRRequestStatus_builder{
					PublicKeyHashToPendingCsr: map[string]*subcav1.PendingCSR{
						pkhUnrelated: subcav1.PendingCSR_builder{
							Status: &status.Status{Code: int32(codes.Internal)},
						}.Build(),
					},
				}.Build())
			}),
			wantErr: "unrequested key not allowed",
		},
		{
			name: "status CSR entries must have a status",
			req: makeReq(func(req *subcav1.PendingCSRRequest) {
				req.SetStatus(subcav1.PendingCSRRequestStatus_builder{
					PublicKeyHashToPendingCsr: map[string]*subcav1.PendingCSR{
						pkh1: nil,
					},
				}.Build())
			}),
			wantErr: "status required",
		},
		{
			name: "status CSR `OK` entries must have a PEM",
			req: makeReq(func(req *subcav1.PendingCSRRequest) {
				req.SetStatus(subcav1.PendingCSRRequestStatus_builder{
					PublicKeyHashToPendingCsr: map[string]*subcav1.PendingCSR{
						pkh1: subcav1.PendingCSR_builder{
							Status: &status.Status{}, // zero == OK
						}.Build(),
					},
				}.Build())
			}),
			wantErr: "csr required",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := service.CreatePendingCSRRequest(t.Context(), test.req)
			assert.ErrorContains(t, err, test.wantErr, "PendingCSRRequest validation error mismatch")
		})
	}
}
