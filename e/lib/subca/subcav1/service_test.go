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

package subcav1_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	subcapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/subca/subcav1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	subcaenv "github.com/gravitational/teleport/lib/subca/testenv"
)

func TestService_authz(t *testing.T) {
	t.Parallel()

	authorizer := &denyAuthorizer{}
	env := subcav1.NewEnv(t, subcav1.EnvParams{
		Authorizer: authorizer,
	})
	subCA := env.SubCAClient

	const caType = string(types.WindowsCA)
	clusterName := env.ClusterName
	validLookingCAOverride := &subcapb.CertAuthorityOverride{
		Kind:    types.KindCertAuthorityOverride,
		SubKind: caType,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: clusterName,
		},
		Spec: &subcapb.CertAuthorityOverrideSpec{
			CertificateOverrides: []*subcapb.CertificateOverride{
				nil, // Passes initial checks, but fails validation.
			},
		},
	}

	tests := []struct {
		name                   string
		doRPC                  func(t *testing.T) error
		want                   []*authorizeAttempt
		adminActionNotRequired bool
	}{
		{
			name: "CreateCertAuthorityOverride",
			doRPC: func(t *testing.T) error {
				_, err := subCA.CreateCertAuthorityOverride(
					t.Context(), &subcapb.CreateCertAuthorityOverrideRequest{
						CaOverride: validLookingCAOverride,
					})
				return err
			},
			want: []*authorizeAttempt{
				{Rule: types.KindCertAuthorityOverride, Verb: types.VerbCreate},
			},
		},
		{
			name: "GetCertAuthorityOverride",
			doRPC: func(t *testing.T) error {
				_, err := subCA.GetCertAuthorityOverride(t.Context(), &subcapb.GetCertAuthorityOverrideRequest{
					CaId: &subcapb.CertAuthorityOverrideID{
						CaType: caType,
					},
				})
				return err
			},
			want: []*authorizeAttempt{
				{Rule: types.KindCertAuthorityOverride, Verb: types.VerbRead},
			},
			adminActionNotRequired: true,
		},
		{
			name: "ListCertAuthorityOverride",
			doRPC: func(t *testing.T) error {
				_, err := subCA.ListCertAuthorityOverride(
					t.Context(), &subcapb.ListCertAuthorityOverrideRequest{})
				return err
			},
			want: []*authorizeAttempt{
				// Order is deterministic.
				{Rule: types.KindCertAuthorityOverride, Verb: types.VerbRead},
				{Rule: types.KindCertAuthorityOverride, Verb: types.VerbList},
			},
			adminActionNotRequired: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Don't t.Parallel(), denyAuthorizer is not built for concurrency.

			authorizer.Reset()
			err := test.doRPC(t)
			require.ErrorAs(t, err, new(*trace.AccessDeniedError), "RPC error mismatch")
			assert.ErrorContains(t, err, "deny authorizer")

			got := authorizer.GetAttemptsAndReset()
			want := test.want
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("Authz attempts mismatch (-want +got)\n%s", diff)
			}

			t.Run("admin actions", func(t *testing.T) {
				if test.adminActionNotRequired {
					authorizer.SetAllowWithAdminAction(authz.AdminActionAuthUnauthorized)
					// Success or a non-AccessDenied error are both valid.
					if err := test.doRPC(t); err != nil {
						assert.NotErrorAs(t, err, new(*trace.AccessDeniedError),
							"Want admin action not required")
					}
					return
				}

				// Admin action requirement fails.
				authorizer.SetAllowWithAdminAction(authz.AdminActionAuthUnauthorized)
				err := test.doRPC(t)
				require.ErrorAs(t, err, new(*trace.AccessDeniedError),
					"Want admin action required")
				require.ErrorContains(t, err, "admin-level API")

				// Admin action requirement fulfilled.
				authorizer.SetAllowWithAdminAction(authz.AdminActionAuthMFAVerifiedWithReuse)
				err = test.doRPC(t)
				assert.NotErrorAs(t, err, new(*trace.AccessDeniedError),
					"Want admin action fulfilled")
			})
		})
	}
}

type authorizeAttempt struct {
	Rule, Verb string
}

// denyAuthorizer is an Authorizer/Checker implementation that denies all
// access.
// All access attempts are recorded so that rules/verbs used by the system can
// be verified.
type denyAuthorizer struct {
	services.AccessChecker

	attempts []*authorizeAttempt

	allowNext        bool
	adminActionState authz.AdminActionAuthState
}

func (a *denyAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	return &authz.Context{
		Checker:              a,
		AdminActionAuthState: a.adminActionState,
	}, nil
}

func (a *denyAuthorizer) CheckAccessToRule(context services.RuleContext, namespace string, rule string, verb string) error {
	a.attempts = append(a.attempts, &authorizeAttempt{
		Rule: rule,
		Verb: verb,
	})

	if a.allowNext {
		return nil
	}

	return trace.AccessDenied("deny authorizer")
}

// GetAttemptsAndReset returns the current authentication attempts and resets
// the authorizer state.
func (a *denyAuthorizer) GetAttemptsAndReset() []*authorizeAttempt {
	ats := a.attempts
	a.Reset()
	return ats
}

// SetAllowWithAdminAction allows following CheckAccessToRule calls to succeed
// and sets its admin action state.
func (a *denyAuthorizer) SetAllowWithAdminAction(s authz.AdminActionAuthState) {
	a.allowNext = true
	a.adminActionState = s
}

func (a *denyAuthorizer) Reset() {
	a.attempts = nil
	a.allowNext = false
	a.adminActionState = 0
}

func TestService_Create(t *testing.T) {
	t.Parallel()

	const caType1 = types.DatabaseClientCA // invalid test
	const caType2 = types.WindowsCA        // success test

	env := subcav1.NewEnv(t, subcav1.EnvParams{
		StorageParams: subcaenv.EnvParams{
			CATypesToCreate: []types.CertAuthType{
				caType1,
				caType2,
			},
		},
	})
	subCA := env.SubCAClient

	t.Run("nil resource", func(t *testing.T) {
		t.Parallel()

		_, err := subCA.CreateCertAuthorityOverride(t.Context(), &subcapb.CreateCertAuthorityOverrideRequest{})
		assert.ErrorContains(t, err, "name required")
	})

	t.Run("invalid cluster name", func(t *testing.T) {
		t.Parallel()

		caOverride := env.NewOverrideForCAType(t, caType1)
		caOverride.Metadata.Name = "badclustername"

		_, err := subCA.CreateCertAuthorityOverride(t.Context(), &subcapb.CreateCertAuthorityOverrideRequest{
			CaOverride: caOverride,
		})
		assert.ErrorContains(t, err, `only "`+env.ClusterName+`" is allowed`)
	})

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		caOverride := env.NewOverrideForCAType(t, caType2)

		// Create resource.
		createResp, err := subCA.CreateCertAuthorityOverride(t.Context(), &subcapb.CreateCertAuthorityOverrideRequest{
			CaOverride: caOverride,
		})
		require.NoError(t, err, "CreateCertAuthorityOverride errored")

		// Assert resource.
		got := createResp.CaOverride
		want := caOverride
		want.Metadata.Revision = got.GetMetadata().GetRevision()
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Fatalf("Create mismatch (-want +got)\n%s", diff)
		}

		// Assert audit.
		emitter := env.MockEmitter
		assertCAOverrideEvent(t, emitter.Events(), &wantEvent{
			Type:    events.CertAuthOverrideCreateEvent,
			Code:    events.CertAuthOverrideCreateCode,
			Success: true,
		})

		// Assert storage.
		getResp, err := subCA.GetCertAuthorityOverride(t.Context(), &subcapb.GetCertAuthorityOverrideRequest{
			CaId: &subcapb.CertAuthorityOverrideID{
				CaType: got.SubKind,
			},
		})
		require.NoError(t, err, "GetCertAuthorityOverride errored")
		want = got
		got = getResp.CaOverride
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("Get mismatch (-want +got)\n%s", diff)
		}
	})
}

func TestService_List(t *testing.T) {
	t.Parallel()

	const caType1 = types.DatabaseClientCA
	const caType2 = types.WindowsCA
	env := subcav1.NewEnv(t, subcav1.EnvParams{
		StorageParams: subcaenv.EnvParams{
			CATypesToCreate: []types.CertAuthType{
				caType1,
				caType2,
			},
		},
	})
	subCA := env.SubCAClient

	t.Run("empty", func(t *testing.T) {
		// Don't t.Parallel(), can't race against override creation.

		got, err := subCA.ListCertAuthorityOverride(
			t.Context(), &subcapb.ListCertAuthorityOverrideRequest{})
		require.NoError(t, err, "List errored")

		// Verify empty response.
		want := &subcapb.ListCertAuthorityOverrideResponse{}
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("List mismatch (-want +got)\n%s", diff)
		}
	})

	// Prepare overrides for testing.
	o1 := env.NewOverrideForCAType(t, caType1)
	resp, err := subCA.CreateCertAuthorityOverride(t.Context(), &subcapb.CreateCertAuthorityOverrideRequest{
		CaOverride: o1,
	})
	require.NoError(t, err, "Create errored")
	o1 = resp.CaOverride

	o2 := env.NewOverrideForCAType(t, caType2)
	resp, err = subCA.CreateCertAuthorityOverride(t.Context(), &subcapb.CreateCertAuthorityOverrideRequest{
		CaOverride: o2,
	})
	require.NoError(t, err, "Create errored")
	o2 = resp.CaOverride

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		resp, err := subCA.ListCertAuthorityOverride(
			t.Context(), &subcapb.ListCertAuthorityOverrideRequest{})
		require.NoError(t, err, "List")
		assert.Empty(t, resp.NextPageToken, "got non-empty nextPageToken")

		got := resp.CaOverrides
		want := []*subcapb.CertAuthorityOverride{o1, o2}
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("List mismatch (-want +got)\n%s", diff)
		}
	})
}

type wantEvent struct {
	Code    string
	Type    string
	Success bool
}

func assertCAOverrideEvent(
	t *testing.T,
	events []apievents.AuditEvent,
	want *wantEvent,
) {
	t.Helper()
	require.Len(t, events, 1, "Number of audit events")

	e := events[0]
	require.IsType(t, &apievents.CertAuthorityOverrideEvent{}, e, "Event type mismatch")
	caoEvent := e.(*apievents.CertAuthorityOverrideEvent)

	assert.Equal(t, want.Type, caoEvent.Type, "Event.Type mismatch")
	assert.Equal(t, want.Code, caoEvent.Code, "Event.Code mismatch")
	assert.Equal(t, want.Success, caoEvent.Success, "Event.Success mismatch")
}
