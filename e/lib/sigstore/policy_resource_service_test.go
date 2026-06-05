package sigstore_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/sigstore"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func Test_PolicyResourceService_CreateSigstorePolicy(t *testing.T) {
	ctx := context.Background()

	t.Run("access denied", func(t *testing.T) {
		service, pack := testService(t)
		pack.authz.deny(types.VerbCreate)

		_, err := service.CreateSigstorePolicy(ctx, workloadidentityv1.CreateSigstorePolicyRequest_builder{
			SigstorePolicy: testPolicy(),
		}.Build())
		require.True(t, trace.IsAccessDenied(err))
	})

	t.Run("mfa required", func(t *testing.T) {
		service, pack := testService(t)
		pack.authz.allow(types.VerbCreate)
		pack.adminActionAuthState = authz.AdminActionAuthUnauthorized

		_, err := service.CreateSigstorePolicy(ctx, workloadidentityv1.CreateSigstorePolicyRequest_builder{
			SigstorePolicy: testPolicy(),
		}.Build())
		require.True(t, trace.IsAccessDenied(err))
	})

	t.Run("success", func(t *testing.T) {
		service, pack := testService(t)
		pack.authz.allow(types.VerbCreate)

		policy := testPolicy()
		created, err := service.CreateSigstorePolicy(ctx, workloadidentityv1.CreateSigstorePolicyRequest_builder{
			SigstorePolicy: policy,
		}.Build())
		require.NoError(t, err)
		assert.Empty(t, cmp.Diff(policy, created, protocmp.Transform()))

		require.Len(t, pack.emitter.Events(), 1)
		event := pack.emitter.LastEvent()
		assert.Equal(t, events.SigstorePolicyCreateEvent, event.GetType())
		assert.Equal(t, events.SigstorePolicyCreateCode, event.GetCode())

		pack.authz.allow(types.VerbRead)
		read, err := service.GetSigstorePolicy(ctx, workloadidentityv1.GetSigstorePolicyRequest_builder{
			Name: policy.GetMetadata().GetName(),
		}.Build())
		require.NoError(t, err)
		assert.Empty(t, cmp.Diff(policy, read, protocmp.Transform()))
	})
}

func Test_PolicyResourceService_GetSigstorePolicy(t *testing.T) {
	ctx := context.Background()

	t.Run("access denied", func(t *testing.T) {
		service, pack := testService(t)
		pack.authz.deny(types.VerbRead)

		_, err := service.GetSigstorePolicy(ctx, workloadidentityv1.GetSigstorePolicyRequest_builder{
			Name: "my-sigstore-policy",
		}.Build())
		require.True(t, trace.IsAccessDenied(err))
	})

	t.Run("missing name", func(t *testing.T) {
		service, pack := testService(t)
		pack.authz.allow(types.VerbRead)

		_, err := service.GetSigstorePolicy(ctx, workloadidentityv1.GetSigstorePolicyRequest_builder{
			Name: "",
		}.Build())
		require.True(t, trace.IsBadParameter(err))
		require.ErrorContains(t, err, "name: must be non-empty")
	})

	t.Run("not found", func(t *testing.T) {
		service, pack := testService(t)
		pack.authz.allow(types.VerbRead)

		_, err := service.GetSigstorePolicy(ctx, workloadidentityv1.GetSigstorePolicyRequest_builder{
			Name: "my-sigstore-policy",
		}.Build())
		require.True(t, trace.IsNotFound(err))
	})
}

func Test_PolicyResourceService_DeleteSigstorePolicy(t *testing.T) {
	ctx := context.Background()

	t.Run("missing name", func(t *testing.T) {
		service, pack := testService(t)
		pack.authz.allow(types.VerbDelete)

		_, err := service.DeleteSigstorePolicy(ctx, workloadidentityv1.DeleteSigstorePolicyRequest_builder{
			Name: "",
		}.Build())
		require.True(t, trace.IsBadParameter(err))
		require.ErrorContains(t, err, "name: must be non-empty")
	})

	t.Run("not found", func(t *testing.T) {
		service, pack := testService(t)
		pack.authz.allow(types.VerbDelete)

		_, err := service.DeleteSigstorePolicy(ctx, workloadidentityv1.DeleteSigstorePolicyRequest_builder{
			Name: "does-not-exist",
		}.Build())
		require.True(t, trace.IsNotFound(err))
	})

	t.Run("access denied", func(t *testing.T) {
		service, pack := testService(t)
		pack.authz.deny(types.VerbDelete)

		_, err := service.DeleteSigstorePolicy(ctx, workloadidentityv1.DeleteSigstorePolicyRequest_builder{
			Name: "my-sigstore-policy",
		}.Build())
		require.True(t, trace.IsAccessDenied(err))
	})

	t.Run("mfa required", func(t *testing.T) {
		service, pack := testService(t)
		pack.authz.allow(types.VerbDelete)
		pack.adminActionAuthState = authz.AdminActionAuthUnauthorized

		_, err := service.DeleteSigstorePolicy(ctx, workloadidentityv1.DeleteSigstorePolicyRequest_builder{
			Name: "my-sigstore-policy",
		}.Build())
		require.True(t, trace.IsAccessDenied(err))
	})

	t.Run("success", func(t *testing.T) {
		service, pack := testService(t)
		pack.authz.allow(types.VerbCreate)

		policy, err := service.CreateSigstorePolicy(ctx, workloadidentityv1.CreateSigstorePolicyRequest_builder{
			SigstorePolicy: testPolicy(),
		}.Build())
		require.NoError(t, err)
		pack.emitter.Reset()

		pack.authz.allow(types.VerbDelete)
		_, err = service.DeleteSigstorePolicy(ctx, workloadidentityv1.DeleteSigstorePolicyRequest_builder{
			Name: policy.GetMetadata().GetName(),
		}.Build())
		require.NoError(t, err)

		require.Len(t, pack.emitter.Events(), 1)
		event := pack.emitter.LastEvent()
		assert.Equal(t, events.SigstorePolicyDeleteEvent, event.GetType())
		assert.Equal(t, events.SigstorePolicyDeleteCode, event.GetCode())

		pack.authz.allow(types.VerbRead)
		_, err = service.GetSigstorePolicy(ctx, workloadidentityv1.GetSigstorePolicyRequest_builder{
			Name: policy.GetMetadata().GetName(),
		}.Build())
		require.True(t, trace.IsNotFound(err))
	})
}

func Test_PolicyResourceService_UpdateSigstorePolicy(t *testing.T) {
	ctx := context.Background()

	t.Run("not found", func(t *testing.T) {
		service, pack := testService(t)
		pack.authz.allow(types.VerbUpdate)

		_, err := service.UpdateSigstorePolicy(ctx, workloadidentityv1.UpdateSigstorePolicyRequest_builder{
			SigstorePolicy: testPolicy(),
		}.Build())
		require.True(t, trace.IsCompareFailed(err))
	})

	t.Run("concurrent update", func(t *testing.T) {
		service, pack := testService(t)
		pack.authz.allow(types.VerbCreate)

		policy, err := service.CreateSigstorePolicy(ctx, workloadidentityv1.CreateSigstorePolicyRequest_builder{
			SigstorePolicy: testPolicy(),
		}.Build())
		require.NoError(t, err)

		policy.GetMetadata().SetRevision("something old")

		pack.authz.allow(types.VerbUpdate)
		_, err = service.UpdateSigstorePolicy(ctx, workloadidentityv1.UpdateSigstorePolicyRequest_builder{
			SigstorePolicy: testPolicy(),
		}.Build())
		require.True(t, trace.IsCompareFailed(err))
	})

	t.Run("access denied", func(t *testing.T) {
		service, pack := testService(t)
		pack.authz.deny(types.VerbUpdate)

		_, err := service.UpdateSigstorePolicy(ctx, workloadidentityv1.UpdateSigstorePolicyRequest_builder{
			SigstorePolicy: testPolicy(),
		}.Build())
		require.True(t, trace.IsAccessDenied(err))
	})

	t.Run("mfa required", func(t *testing.T) {
		service, pack := testService(t)
		pack.authz.allow(types.VerbUpdate)
		pack.adminActionAuthState = authz.AdminActionAuthUnauthorized

		_, err := service.UpdateSigstorePolicy(ctx, workloadidentityv1.UpdateSigstorePolicyRequest_builder{
			SigstorePolicy: testPolicy(),
		}.Build())
		require.True(t, trace.IsAccessDenied(err))
	})

	t.Run("success", func(t *testing.T) {
		service, pack := testService(t)
		pack.authz.allow(types.VerbCreate)

		policy, err := service.CreateSigstorePolicy(ctx, workloadidentityv1.CreateSigstorePolicyRequest_builder{
			SigstorePolicy: testPolicy(),
		}.Build())
		require.NoError(t, err)
		pack.emitter.Reset()

		policy.GetSpec().GetKeyless().GetIdentities()[0].SetSubject("new-subject")

		pack.authz.allow(types.VerbUpdate)
		_, err = service.UpdateSigstorePolicy(ctx, workloadidentityv1.UpdateSigstorePolicyRequest_builder{
			SigstorePolicy: policy,
		}.Build())
		require.NoError(t, err)

		require.Len(t, pack.emitter.Events(), 1)
		event := pack.emitter.LastEvent()
		assert.Equal(t, events.SigstorePolicyUpdateEvent, event.GetType())
		assert.Equal(t, events.SigstorePolicyUpdateCode, event.GetCode())
	})
}

func Test_PolicyResourceService_UpsertSigstorePolicy(t *testing.T) {
	ctx := context.Background()

	t.Run("access denied", func(t *testing.T) {
		service, pack := testService(t)
		pack.authz.deny(types.VerbCreate)
		pack.authz.deny(types.VerbUpdate)

		_, err := service.UpsertSigstorePolicy(ctx, workloadidentityv1.UpsertSigstorePolicyRequest_builder{
			SigstorePolicy: testPolicy(),
		}.Build())
		require.True(t, trace.IsAccessDenied(err))
	})

	t.Run("mfa required", func(t *testing.T) {
		service, pack := testService(t)
		pack.authz.allow(types.VerbCreate)
		pack.authz.allow(types.VerbUpdate)
		pack.adminActionAuthState = authz.AdminActionAuthUnauthorized

		_, err := service.UpsertSigstorePolicy(ctx, workloadidentityv1.UpsertSigstorePolicyRequest_builder{
			SigstorePolicy: testPolicy(),
		}.Build())
		require.True(t, trace.IsAccessDenied(err))
	})

	t.Run("success", func(t *testing.T) {
		service, pack := testService(t)
		pack.authz.allow(types.VerbCreate)
		pack.authz.allow(types.VerbUpdate)

		created, err := service.UpsertSigstorePolicy(ctx, workloadidentityv1.UpsertSigstorePolicyRequest_builder{
			SigstorePolicy: testPolicy(),
		}.Build())
		require.NoError(t, err)

		created.GetSpec().GetKeyless().GetIdentities()[0].SetSubject("new-subject")
		created.GetMetadata().SetRevision("something old")

		updated, err := service.UpsertSigstorePolicy(ctx, workloadidentityv1.UpsertSigstorePolicyRequest_builder{
			SigstorePolicy: created,
		}.Build())
		require.NoError(t, err)
		assert.Empty(t, cmp.Diff(updated.GetSpec(), created.GetSpec(), protocmp.Transform()))

		require.Len(t, pack.emitter.Events(), 2)
		for _, event := range pack.emitter.Events() {
			assert.Equal(t, events.SigstorePolicyCreateEvent, event.GetType())
			assert.Equal(t, events.SigstorePolicyCreateCode, event.GetCode())
		}
	})
}

func Test_PolicyResourceService_ListSigstorePolicies(t *testing.T) {
	ctx := context.Background()

	t.Run("access denied", func(t *testing.T) {
		service, pack := testService(t)
		pack.authz.deny(types.VerbList)
		pack.authz.deny(types.VerbRead)

		_, err := service.ListSigstorePolicies(ctx, &workloadidentityv1.ListSigstorePoliciesRequest{})
		require.True(t, trace.IsAccessDenied(err))
	})

	t.Run("success", func(t *testing.T) {
		service, pack := testService(t)
		pack.authz.allow(types.VerbCreate)

		for i := range 2 {
			policy := testPolicy()
			policy.GetMetadata().SetName(fmt.Sprintf("policy-%d", i))

			_, err := service.CreateSigstorePolicy(ctx, workloadidentityv1.CreateSigstorePolicyRequest_builder{
				SigstorePolicy: policy,
			}.Build())
			require.NoError(t, err)
		}

		pack.authz.allow(types.VerbRead)
		pack.authz.allow(types.VerbList)
		rsp, err := service.ListSigstorePolicies(ctx, workloadidentityv1.ListSigstorePoliciesRequest_builder{
			PageSize: 1,
		}.Build())
		require.NoError(t, err)
		require.Len(t, rsp.GetSigstorePolicies(), 1)
		assert.Equal(t, "policy-0", rsp.GetSigstorePolicies()[0].GetMetadata().GetName())
		require.NotEmpty(t, rsp.GetNextPageToken())

		rsp, err = service.ListSigstorePolicies(ctx, workloadidentityv1.ListSigstorePoliciesRequest_builder{
			PageSize:  1,
			PageToken: rsp.GetNextPageToken(),
		}.Build())
		require.NoError(t, err)
		require.Len(t, rsp.GetSigstorePolicies(), 1)
		assert.Equal(t, "policy-1", rsp.GetSigstorePolicies()[0].GetMetadata().GetName())
		assert.Empty(t, rsp.GetNextPageToken())
	})
}

func testPolicy() *workloadidentityv1.SigstorePolicy {
	return workloadidentityv1.SigstorePolicy_builder{
		Kind:    types.KindSigstorePolicy,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: "github-provenance",
		}.Build(),
		Spec: workloadidentityv1.SigstorePolicySpec_builder{
			Keyless: workloadidentityv1.SigstoreKeylessAuthority_builder{
				Identities: []*workloadidentityv1.SigstoreKeylessSigningIdentity{
					workloadidentityv1.SigstoreKeylessSigningIdentity_builder{
						Issuer:       proto.String("https://token.actions.githubusercontent.com"),
						SubjectRegex: proto.String(`https://github.com/mycompany/.*/\.github/workflows/.*@.*`),
					}.Build(),
				},
			}.Build(),
			Requirements: workloadidentityv1.SigstorePolicyRequirements_builder{
				Attestations: []*workloadidentityv1.InTotoAttestationMatcher{
					workloadidentityv1.InTotoAttestationMatcher_builder{PredicateType: "https://slsa.dev/provenance/v1"}.Build(),
				},
			}.Build(),
		}.Build(),
	}.Build()
}

func testService(t *testing.T) (*sigstore.PolicyResourceService, *testPack) {
	t.Helper()

	mem, err := memory.New(memory.Config{
		Context: context.Background(),
	})
	require.NoError(t, err)

	backend, err := local.NewSigstorePolicyService(mem)
	require.NoError(t, err)

	pack := newTestPack(t)

	srv, err := sigstore.NewPolicyResourceService(sigstore.PolicyResourceServiceConfig{
		Backend: backend,
		Authorizer: authz.AuthorizerFunc(func(context.Context) (*authz.Context, error) {
			return &authz.Context{
				Checker:              pack.authz,
				AdminActionAuthState: pack.adminActionAuthState,
			}, nil
		}),
		Logger:  logtest.NewLogger(),
		Emitter: pack.emitter,
	})
	require.NoError(t, err)

	return srv, pack
}

type testPack struct {
	emitter              *eventstest.MockRecorderEmitter
	authz                *mockChecker
	adminActionAuthState authz.AdminActionAuthState
}

func newTestPack(t *testing.T) *testPack {
	t.Helper()

	return &testPack{
		emitter:              &eventstest.MockRecorderEmitter{},
		authz:                &mockChecker{},
		adminActionAuthState: authz.AdminActionAuthMFAVerified,
	}
}

type mockChecker struct {
	services.AccessChecker
	mock.Mock
}

func (c *mockChecker) CheckAccessToRule(context services.RuleContext, namespace string, rule string, verb string) error {
	return c.Called(context, namespace, rule, verb).Error(0)
}

func (c *mockChecker) allow(verb string) {
	c.On(
		"CheckAccessToRule",
		mock.Anything,
		"default",
		types.KindSigstorePolicy,
		verb,
	).Return(nil)
}

func (c *mockChecker) deny(verb string) {
	c.On(
		"CheckAccessToRule",
		mock.Anything,
		"default",
		types.KindSigstorePolicy,
		verb,
	).Return(
		trace.AccessDenied("access denied"),
	)
}
