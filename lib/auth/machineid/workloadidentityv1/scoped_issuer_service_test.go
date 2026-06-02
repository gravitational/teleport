// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package workloadidentityv1_test

import (
	"crypto/x509"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/cryptosuites"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
)

// newScopedIssuanceUser creates a scoped user assigned a scoped role that grants
// issuance using WorkloadIdentity resources within the given scope: it holds
// read_no_secrets+list rules for the workload_identity kind and a
// workload_identity label selector. The returned username can be used with
// authtest.TestScopedUser to mint a scoped client pinned to the scope.
func newScopedIssuanceUser(
	t *testing.T,
	srv *authtest.TLSServer,
	adminClient *authclient.Client,
	username string,
	scope string,
	issuanceLabels []*labelv1.Label,
) string {
	t.Helper()
	ctx := t.Context()

	scopedSvc := adminClient.ScopedAccessServiceClient()
	role, err := scopedSvc.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
		Role: &scopedaccessv1.ScopedRole{
			Kind:    scopedaccess.KindScopedRole,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: username + "-role",
			},
			Scope: "/scopes",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{scope},
				Rules: []*scopedaccessv1.ScopedRule{
					{
						Resources: []string{types.KindWorkloadIdentity},
						Verbs: []string{
							types.VerbReadNoSecrets,
							types.VerbList,
						},
					},
				},
				WorkloadIdentity: &scopedaccessv1.ScopedRoleWorkloadIdentity{
					Labels: issuanceLabels,
				},
			},
		},
	})
	require.NoError(t, err)

	user, err := authtest.CreateUser(ctx, srv.Auth(), username)
	require.NoError(t, err)

	resp, err := scopedSvc.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: &scopedaccessv1.ScopedRoleAssignment{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: uuid.NewString(),
			},
			Scope: "/scopes",
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				User: user.GetName(),
				Assignments: []*scopedaccessv1.Assignment{
					{Role: role.Role.Metadata.Name, Scope: scope},
				},
			},
		},
	})
	require.NoError(t, err)

	// Wait for the assignment to propagate to the cache used by the authorizer.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		_, err := srv.Auth().ScopedAccessCache.GetScopedRoleAssignment(
			ctx, &scopedaccessv1.GetScopedRoleAssignmentRequest{
				Name:    resp.GetAssignment().GetMetadata().GetName(),
				SubKind: resp.GetAssignment().GetSubKind(),
			})
		require.NoError(t, err)
	}, 10*time.Second, 100*time.Millisecond)

	return user.GetName()
}

// createScopedWorkloadIdentityWithLabels creates a scoped WorkloadIdentity (via
// the backend, bypassing RPC authorization) with the given resource labels.
func createScopedWorkloadIdentityWithLabels(
	t *testing.T,
	srv *authtest.TLSServer,
	name, scope, spiffeID string,
	labels map[string]string,
) {
	t.Helper()
	_, err := srv.Auth().CreateWorkloadIdentity(t.Context(), &workloadidentityv1pb.WorkloadIdentity{
		Kind:    types.KindWorkloadIdentity,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:   name,
			Labels: labels,
		},
		Scope: scope,
		Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
			Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
				Id: spiffeID,
			},
		},
	})
	require.NoError(t, err)
}

func TestIssuanceService_ScopedIdentity(t *testing.T) {
	t.Setenv("TELEPORT_UNSTABLE_SCOPES", "yes")
	ctx := t.Context()
	tp := newIssuanceTestPack(t, ctx)
	srv := tp.srv

	adminClient, err := srv.NewClient(authtest.TestAdmin())
	require.NoError(t, err)
	t.Cleanup(func() { _ = adminClient.Close() })

	const grantedScope = "/scopes/granted"
	const otherScope = "/scopes/other"
	prodLabels := []*labelv1.Label{{Name: "env", Values: []string{"prod"}}}

	// The granted user may issue prod-labelled WorkloadIdentities in
	// /scopes/granted.
	grantedUser := newScopedIssuanceUser(t, srv, adminClient, "issue-granted", grantedScope, prodLabels)
	grantedClient, err := srv.NewClient(authtest.TestScopedUser(grantedUser, grantedScope))
	require.NoError(t, err)
	t.Cleanup(func() { _ = grantedClient.Close() })
	grantedIssue := workloadidentityv1pb.NewWorkloadIdentityIssuanceServiceClient(grantedClient.GetConnection())

	// WorkloadIdentities used across the subtests.
	createScopedWorkloadIdentityWithLabels(t, srv,
		"prod-svc", grantedScope, grantedScope+"/_/prod-svc",
		map[string]string{"env": "prod", "name": "prod-svc"})
	createScopedWorkloadIdentityWithLabels(t, srv,
		"dev-svc", grantedScope, grantedScope+"/_/dev-svc",
		map[string]string{"env": "dev"})
	createScopedWorkloadIdentityWithLabels(t, srv,
		"other-svc", otherScope, otherScope+"/_/other-svc",
		map[string]string{"env": "prod"})
	// A templated SPIFFE ID whose admin section is driven by a client-supplied
	// (attacker-controllable) workload attribute. The unrendered form is valid;
	// the rendered form may escape the scope.
	createScopedWorkloadIdentityWithLabels(t, srv,
		"bypass-svc", grantedScope, grantedScope+"/_/{{ workload.kubernetes.namespace }}",
		map[string]string{"env": "prod", "name": "bypass-svc"})

	// Generate a keypair to request X509 SVIDs for.
	workloadKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	workloadKeyPubBytes, err := x509.MarshalPKIXPublicKey(workloadKey.Public())
	require.NoError(t, err)

	x509Cred := &workloadidentityv1pb.IssueWorkloadIdentityRequest_X509SvidParams{
		X509SvidParams: &workloadidentityv1pb.X509SVIDParams{
			PublicKey: workloadKeyPubBytes,
		},
	}
	benignNamespace := func() *workloadidentityv1pb.WorkloadAttrs {
		return &workloadidentityv1pb.WorkloadAttrs{
			Kubernetes: &workloadidentityv1pb.WorkloadAttrsKubernetes{
				Attested:  true,
				Namespace: "default",
			},
		}
	}

	t.Run("issue success", func(t *testing.T) {
		res, err := grantedIssue.IssueWorkloadIdentity(ctx, &workloadidentityv1pb.IssueWorkloadIdentityRequest{
			Name:       "prod-svc",
			Credential: x509Cred,
		})
		require.NoError(t, err)
		require.Equal(t,
			"spiffe://localhost"+grantedScope+"/_/prod-svc",
			res.GetCredential().GetSpiffeId(),
		)
	})

	t.Run("issue denied for WorkloadIdentity in another scope", func(t *testing.T) {
		_, err := grantedIssue.IssueWorkloadIdentity(ctx, &workloadidentityv1pb.IssueWorkloadIdentityRequest{
			Name:       "other-svc",
			Credential: x509Cred,
		})
		require.True(t, trace.IsAccessDenied(err), "expected AccessDenied, got %v", err)
	})

	t.Run("issue denied for label mismatch", func(t *testing.T) {
		_, err := grantedIssue.IssueWorkloadIdentity(ctx, &workloadidentityv1pb.IssueWorkloadIdentityRequest{
			Name:       "dev-svc",
			Credential: x509Cred,
		})
		require.True(t, trace.IsAccessDenied(err), "expected AccessDenied, got %v", err)
	})

	t.Run("issue rejects templating that escapes scope", func(t *testing.T) {
		// The rendered SPIFFE ID becomes
		// /scopes/granted/_/escaped/_/elevated, which contains a second scope
		// separator and is rejected by the issuance-time re-validation.
		_, err := grantedIssue.IssueWorkloadIdentity(ctx, &workloadidentityv1pb.IssueWorkloadIdentityRequest{
			Name:       "bypass-svc",
			Credential: x509Cred,
			WorkloadAttrs: &workloadidentityv1pb.WorkloadAttrs{
				Kubernetes: &workloadidentityv1pb.WorkloadAttrsKubernetes{
					Attested:  true,
					Namespace: "escaped/_/elevated",
				},
			},
		})
		require.True(t, trace.IsBadParameter(err), "expected BadParameter, got %v", err)
		require.ErrorContains(t, err, "rendered SPIFFE ID")
	})

	t.Run("issue allows templating that stays within scope", func(t *testing.T) {
		res, err := grantedIssue.IssueWorkloadIdentity(ctx, &workloadidentityv1pb.IssueWorkloadIdentityRequest{
			Name:          "bypass-svc",
			Credential:    x509Cred,
			WorkloadAttrs: benignNamespace(),
		})
		require.NoError(t, err)
		require.Equal(t,
			"spiffe://localhost"+grantedScope+"/_/default",
			res.GetCredential().GetSpiffeId(),
		)
	})

	t.Run("issue_identities filters by scope and labels", func(t *testing.T) {
		res, err := grantedIssue.IssueWorkloadIdentities(ctx, &workloadidentityv1pb.IssueWorkloadIdentitiesRequest{
			LabelSelectors: []*workloadidentityv1pb.LabelSelector{
				{Key: "env", Values: []string{"prod"}},
			},
			Credential: &workloadidentityv1pb.IssueWorkloadIdentitiesRequest_X509SvidParams{
				X509SvidParams: &workloadidentityv1pb.X509SVIDParams{
					PublicKey: workloadKeyPubBytes,
				},
			},
			WorkloadAttrs: benignNamespace(),
		})
		require.NoError(t, err)

		ids := map[string]struct{}{}
		for _, cred := range res.GetCredentials() {
			ids[cred.GetSpiffeId()] = struct{}{}
		}
		// prod-svc and bypass-svc are in the granted scope and match env=prod.
		require.Contains(t, ids, "spiffe://localhost"+grantedScope+"/_/prod-svc")
		require.Contains(t, ids, "spiffe://localhost"+grantedScope+"/_/default")
		// other-svc is in a scope the caller cannot access and must not leak.
		require.NotContains(t, ids, "spiffe://localhost"+otherScope+"/_/other-svc")
	})

	t.Run("issue_identities filtered to a single label", func(t *testing.T) {
		res, err := grantedIssue.IssueWorkloadIdentities(ctx, &workloadidentityv1pb.IssueWorkloadIdentitiesRequest{
			LabelSelectors: []*workloadidentityv1pb.LabelSelector{
				{Key: "name", Values: []string{"prod-svc"}},
			},
			Credential: &workloadidentityv1pb.IssueWorkloadIdentitiesRequest_X509SvidParams{
				X509SvidParams: &workloadidentityv1pb.X509SVIDParams{
					PublicKey: workloadKeyPubBytes,
				},
			},
		})
		require.NoError(t, err)
		require.Len(t, res.GetCredentials(), 1)
		require.Equal(t,
			"spiffe://localhost"+grantedScope+"/_/prod-svc",
			res.GetCredentials()[0].GetSpiffeId(),
		)
	})
}
