package summarizerv1

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/summarizer"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/session"
)

type testPlugin struct{}

func (p *testPlugin) GetName() string {
	return "auth.enterprise"
}

func (p *testPlugin) RegisterProxyWebHandlers(_ any) error { return nil }
func (p *testPlugin) RegisterAuthWebHandlers(_ any) error  { return nil }

func (p *testPlugin) RegisterAuthServices(
	ctx context.Context, anyServer any, getClientCert func() (*tls.Certificate, error),
) error {
	authServer, ok := anyServer.(*auth.GRPCServer)
	if !ok {
		return trace.BadParameter("expected *auth.GRPCServer, got %T", anyServer)
	}

	svc, err := NewService(ServiceConfig{
		Authorizer:        authServer.Authorizer,
		Backend:           authServer.AuthServer,
		SummaryDownloader: authServer.AuthServer,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	srv, err := authServer.GetServer()
	if err != nil {
		return trace.Wrap(err)
	}

	summarizerv1pb.RegisterSummarizerServiceServer(srv, svc)
	return nil
}

func newTestTLSServer(t testing.TB) *authtest.TLSServer {
	as, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir: t.TempDir(),
	})
	require.NoError(t, err)

	srv, err := as.NewTestTLSServer(func(cfg *authtest.TLSServerConfig) {
		cfg.APIConfig.PluginRegistry = plugin.NewRegistry()
		err = cfg.APIConfig.PluginRegistry.Add(&testPlugin{})
		require.NoError(t, err)
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		err := srv.Close()
		if errors.Is(err, net.ErrClosed) {
			return
		}
		require.NoError(t, err)
	})

	return srv
}

// createTestUser creates a user that has access to all the configuration
// objects and read-only access to only their sessions (using a Where filter).
func createTestUser(
	t *testing.T, srv *authtest.TLSServer, name string, opts ...authtest.CreateUserAndRoleOption,
) types.User {
	user, _, err := authtest.CreateUserAndRole(
		srv.Auth(),
		name,
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindInferenceSecret, types.KindInferenceModel, types.KindInferencePolicy},
				Verbs:     []string{types.VerbCreate, types.VerbRead, types.VerbUpdate, types.VerbDelete, types.VerbList},
			},
			{
				Resources: []string{types.KindSession},
				Verbs:     []string{types.VerbRead},
				Where:     "contains(session.participants, user.metadata.name)",
			},
		},
		opts...,
	)
	require.NoError(t, err)
	return user
}

func newTestResources(t *testing.T, suffix string) (
	*summarizerv1pb.InferenceSecret,
	*summarizerv1pb.InferenceModel,
	*summarizerv1pb.InferencePolicy,
) {
	secret := summarizer.NewInferenceSecret("secret"+suffix, &summarizerv1pb.InferenceSecretSpec{
		Value: "my-secret-value",
	})

	model := summarizer.NewInferenceModel("model"+suffix, &summarizerv1pb.InferenceModelSpec{
		Provider: &summarizerv1pb.InferenceModelSpec_Openai{
			Openai: &summarizerv1pb.OpenAIProvider{
				OpenaiModelId:   "gpt-4o",
				ApiKeySecretRef: "secret" + suffix,
			},
		},
	})

	policy := summarizer.NewInferencePolicy("policy"+suffix, &summarizerv1pb.InferencePolicySpec{
		Kinds: []string{string(types.SSHSessionKind)},
		Model: "model" + suffix,
	})

	return secret, model, policy
}

// assertResourceEquals asserts that two resources are equal, ignoring the
// revision field.
func assertResourceEquals[T types.Resource153](t *testing.T, expected T, actual T) {
	t.Helper()
	assert.Empty(t, cmp.Diff(
		expected,
		actual,
		protocmp.Transform(),
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
	))
}

// assertResourceListEqualsIgnoringOrder asserts that two lists of resources
// are equal, ignoring the revision field and the order of the resources.
func assertResourceListEqualsIgnoringOrder[T types.Resource153](
	t *testing.T,
	expected []T,
	actual []T,
) {
	t.Helper()
	assert.Empty(t, cmp.Diff(expected, actual,
		protocmp.Transform(),
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		cmpopts.SortSlices(func(a, b T) bool {
			return a.GetMetadata().GetName() < b.GetMetadata().GetName()
		}),
	))
}

func TestService_CreateAndGet(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	srv := newTestTLSServer(t)
	user := createTestUser(t, srv, "test-user")

	clt, err := srv.NewClient(authtest.TestUser(user.GetName()))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()

	secret, expectedModel, expectedPolicy := newTestResources(t, "1")

	// Test creating resources.
	createdSecret, err := sclt.CreateInferenceSecret(ctx, &summarizerv1pb.CreateInferenceSecretRequest{
		Secret: secret,
	})
	require.NoError(t, err)
	assertResourceEquals(t, secret, createdSecret.Secret)

	// The secret value should have been written to the backend (we later test
	// retrieving through gRPC, but there we don't expect the secret value to be
	// returned).
	backendSecret, err := srv.AuthServer.AuthServer.GetInferenceSecret(ctx, "secret1")
	require.NoError(t, err)
	assertResourceEquals(t, secret, backendSecret)

	createdModel, err := sclt.CreateInferenceModel(ctx, &summarizerv1pb.CreateInferenceModelRequest{
		Model: expectedModel,
	})
	require.NoError(t, err)
	assertResourceEquals(t, expectedModel, createdModel.Model)

	createdPolicy, err := sclt.CreateInferencePolicy(ctx, &summarizerv1pb.CreateInferencePolicyRequest{
		Policy: expectedPolicy,
	})
	require.NoError(t, err)
	assertResourceEquals(t, expectedPolicy, createdPolicy.Policy)

	// Test retrieving resources.
	gotSecret, err := sclt.GetInferenceSecret(ctx, &summarizerv1pb.GetInferenceSecretRequest{
		Name: "secret1",
	})
	require.NoError(t, err)

	// The returned secret should not contain the spec, as it is sensitive
	// information.
	expectedSecret := proto.CloneOf(secret)
	expectedSecret.Spec = nil
	assertResourceEquals(t, expectedSecret, gotSecret.Secret)

	gotModel, err := sclt.GetInferenceModel(ctx, &summarizerv1pb.GetInferenceModelRequest{
		Name: "model1",
	})
	require.NoError(t, err)
	assertResourceEquals(t, expectedModel, gotModel.Model)
}

func TestService_Update(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	srv := newTestTLSServer(t)
	user := createTestUser(t, srv, "test-user")

	clt, err := srv.NewClient(authtest.TestUser(user.GetName()))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()

	secret, expectedModel, expectedPolicy := newTestResources(t, "1")

	// Create the resources.
	createdSecret, err := sclt.CreateInferenceSecret(ctx, &summarizerv1pb.CreateInferenceSecretRequest{
		Secret: secret,
	})
	require.NoError(t, err)

	createdModel, err := sclt.CreateInferenceModel(ctx, &summarizerv1pb.CreateInferenceModelRequest{
		Model: expectedModel,
	})
	require.NoError(t, err)

	createdPolicy, err := sclt.CreateInferencePolicy(ctx, &summarizerv1pb.CreateInferencePolicyRequest{
		Policy: expectedPolicy,
	})
	require.NoError(t, err)

	// Test updating the secret.
	createdSecret.Secret.Spec.Value = "updated-secret-value"
	updatedSecret, err := sclt.UpdateInferenceSecret(ctx, &summarizerv1pb.UpdateInferenceSecretRequest{
		Secret: proto.CloneOf(createdSecret.Secret),
	})
	require.NoError(t, err)

	// The secret should be updated in the backend.
	expectedSecret := proto.CloneOf(createdSecret.Secret)
	backendSecret, err := srv.AuthServer.AuthServer.GetInferenceSecret(ctx, "secret1")
	require.NoError(t, err)
	assertResourceEquals(t, expectedSecret, backendSecret)

	// The secret returned from the RPC should not contain the spec, as it is
	// sensitive information.
	expectedSecret.Spec = nil
	assertResourceEquals(t, expectedSecret, updatedSecret.Secret)

	gotSecret, err := sclt.GetInferenceSecret(ctx, &summarizerv1pb.GetInferenceSecretRequest{
		Name: "secret1",
	})
	require.NoError(t, err)
	assertResourceEquals(t, expectedSecret, gotSecret.Secret)

	// Test updating the model.
	createdModel.Model.Spec.GetOpenai().Temperature = 1.5
	updatedModel, err := sclt.UpdateInferenceModel(ctx, &summarizerv1pb.UpdateInferenceModelRequest{
		Model: proto.CloneOf(createdModel.Model),
	})
	require.NoError(t, err)
	assertResourceEquals(t, createdModel.Model, updatedModel.Model)

	gotModel, err := sclt.GetInferenceModel(ctx, &summarizerv1pb.GetInferenceModelRequest{
		Name: "model1",
	})
	require.NoError(t, err)
	assertResourceEquals(t, createdModel.Model, gotModel.Model)

	// Test updating the policy.
	createdPolicy.Policy.Spec.Filter = `equals(resource.metadata.labels["env"], "dev")`
	updatedPolicy, err := sclt.UpdateInferencePolicy(ctx, &summarizerv1pb.UpdateInferencePolicyRequest{
		Policy: proto.CloneOf(createdPolicy.Policy),
	})
	require.NoError(t, err)
	assertResourceEquals(t, createdPolicy.Policy, updatedPolicy.Policy)

	gotPolicy, err := sclt.GetInferencePolicy(ctx, &summarizerv1pb.GetInferencePolicyRequest{
		Name: "policy1",
	})
	require.NoError(t, err)
	assertResourceEquals(t, createdPolicy.Policy, gotPolicy.Policy)
}

func TestService_Upsert(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	srv := newTestTLSServer(t)
	user := createTestUser(t, srv, "test-user")

	clt, err := srv.NewClient(authtest.TestUser(user.GetName()))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()

	secret, expectedModel, expectedPolicy := newTestResources(t, "1")

	// Test creating by upserting resources.
	createdSecret, err := sclt.UpsertInferenceSecret(ctx, &summarizerv1pb.UpsertInferenceSecretRequest{
		Secret: secret,
	})
	require.NoError(t, err)

	// The secret returned from the RPC should not contain the spec, as it is
	// sensitive information.
	expectedSecret := proto.CloneOf(secret)
	expectedSecret.Spec = nil
	assertResourceEquals(t, expectedSecret, createdSecret.Secret)

	createdModel, err := sclt.UpsertInferenceModel(ctx, &summarizerv1pb.UpsertInferenceModelRequest{
		Model: expectedModel,
	})
	require.NoError(t, err)

	createdPolicy, err := sclt.UpsertInferencePolicy(ctx, &summarizerv1pb.UpsertInferencePolicyRequest{
		Policy: expectedPolicy,
	})
	require.NoError(t, err)

	// Test updating the secret.
	secret.Spec.Value = "updated-secret-value"
	updatedSecret, err := sclt.UpsertInferenceSecret(ctx, &summarizerv1pb.UpsertInferenceSecretRequest{
		Secret: proto.CloneOf(secret),
	})
	require.NoError(t, err)
	expectedSecret = proto.CloneOf(secret)
	expectedSecret.Spec = nil
	assertResourceEquals(t, expectedSecret, updatedSecret.Secret)

	// The secret should be updated in the backend.
	backendSecret, err := srv.AuthServer.AuthServer.GetInferenceSecret(ctx, "secret1")
	require.NoError(t, err)
	assertResourceEquals(t, secret, backendSecret)

	gotSecret, err := sclt.GetInferenceSecret(ctx, &summarizerv1pb.GetInferenceSecretRequest{
		Name: "secret1",
	})
	require.NoError(t, err)
	assertResourceEquals(t, expectedSecret, gotSecret.Secret)

	// Test updating the model.
	createdModel.Model.Spec.GetOpenai().Temperature = 1.5
	updatedModel, err := sclt.UpsertInferenceModel(ctx, &summarizerv1pb.UpsertInferenceModelRequest{
		Model: proto.CloneOf(createdModel.Model),
	})
	require.NoError(t, err)
	assertResourceEquals(t, createdModel.Model, updatedModel.Model)

	gotModel, err := sclt.GetInferenceModel(ctx, &summarizerv1pb.GetInferenceModelRequest{
		Name: "model1",
	})
	require.NoError(t, err)
	assertResourceEquals(t, createdModel.Model, gotModel.Model)

	// Test updating the policy.
	createdPolicy.Policy.Spec.Filter = `equals(resource.metadata.labels["env"], "dev")`
	updatedPolicy, err := sclt.UpsertInferencePolicy(ctx, &summarizerv1pb.UpsertInferencePolicyRequest{
		Policy: proto.CloneOf(createdPolicy.Policy),
	})
	require.NoError(t, err)
	assertResourceEquals(t, createdPolicy.Policy, updatedPolicy.Policy)

	gotPolicy, err := sclt.GetInferencePolicy(ctx, &summarizerv1pb.GetInferencePolicyRequest{
		Name: "policy1",
	})
	require.NoError(t, err)
	assertResourceEquals(t, createdPolicy.Policy, gotPolicy.Policy)
}

func TestService_Delete(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	srv := newTestTLSServer(t)
	user := createTestUser(t, srv, "test-user")

	clt, err := srv.NewClient(authtest.TestUser(user.GetName()))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()

	secret, expectedModel, expectedPolicy := newTestResources(t, "1")

	// Create the resources.
	_, err = sclt.CreateInferenceSecret(ctx, &summarizerv1pb.CreateInferenceSecretRequest{
		Secret: secret,
	})
	require.NoError(t, err)

	_, err = sclt.CreateInferenceModel(ctx, &summarizerv1pb.CreateInferenceModelRequest{
		Model: expectedModel,
	})
	require.NoError(t, err)

	_, err = sclt.CreateInferencePolicy(ctx, &summarizerv1pb.CreateInferencePolicyRequest{
		Policy: expectedPolicy,
	})
	require.NoError(t, err)

	// Delete the resources.
	_, err = sclt.DeleteInferencePolicy(ctx, &summarizerv1pb.DeleteInferencePolicyRequest{
		Name: "policy1",
	})
	require.NoError(t, err)

	_, err = sclt.DeleteInferenceModel(ctx, &summarizerv1pb.DeleteInferenceModelRequest{
		Name: "model1",
	})
	require.NoError(t, err)

	_, err = sclt.DeleteInferenceSecret(ctx, &summarizerv1pb.DeleteInferenceSecretRequest{
		Name: "secret1",
	})
	require.NoError(t, err)

	// Verify that the resources are deleted.
	_, err = sclt.GetInferencePolicy(ctx, &summarizerv1pb.GetInferencePolicyRequest{
		Name: "policy1",
	})
	require.Error(t, err)

	assert.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)
	_, err = sclt.GetInferenceModel(ctx, &summarizerv1pb.GetInferenceModelRequest{
		Name: "model1",
	})
	require.Error(t, err)

	assert.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)
	_, err = sclt.GetInferenceSecret(ctx, &summarizerv1pb.GetInferenceSecretRequest{
		Name: "secret1",
	})
	require.Error(t, err)
	assert.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)
}

func TestService_List(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	srv := newTestTLSServer(t)
	user := createTestUser(t, srv, "test-user")

	clt, err := srv.NewClient(authtest.TestUser(user.GetName()))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()

	expectedSecrets := []*summarizerv1pb.InferenceSecret{}
	expectedModels := []*summarizerv1pb.InferenceModel{}
	expectedPolicies := []*summarizerv1pb.InferencePolicy{}

	// Create resources.
	for i := range 3 {
		secret, expectedModel, expectedPolicy := newTestResources(t, strconv.Itoa(i+1))

		_, err = sclt.CreateInferenceSecret(ctx, &summarizerv1pb.CreateInferenceSecretRequest{
			Secret: secret,
		})
		require.NoError(t, err)
		secret.Spec = nil // The secret spec is sensitive and should not be returned in the list.
		expectedSecrets = append(expectedSecrets, secret)

		_, err = sclt.CreateInferenceModel(ctx, &summarizerv1pb.CreateInferenceModelRequest{
			Model: expectedModel,
		})
		require.NoError(t, err)
		expectedModels = append(expectedModels, expectedModel)

		_, err = sclt.CreateInferencePolicy(ctx, &summarizerv1pb.CreateInferencePolicyRequest{
			Policy: expectedPolicy,
		})
		require.NoError(t, err)
		expectedPolicies = append(expectedPolicies, expectedPolicy)
	}

	// List secrets.
	allSecrets := []*summarizerv1pb.InferenceSecret{}
	secretsPage, err := sclt.ListInferenceSecrets(ctx, &summarizerv1pb.ListInferenceSecretsRequest{
		PageSize: 2,
	})
	require.NoError(t, err)
	assert.Len(t, secretsPage.Secrets, 2)
	allSecrets = append(allSecrets, secretsPage.Secrets...)

	secretsPage, err = sclt.ListInferenceSecrets(ctx, &summarizerv1pb.ListInferenceSecretsRequest{
		PageSize:  2,
		PageToken: secretsPage.NextPageToken,
	})
	require.NoError(t, err)
	assert.Len(t, secretsPage.Secrets, 1)
	allSecrets = append(allSecrets, secretsPage.Secrets...)

	assertResourceListEqualsIgnoringOrder(t, expectedSecrets, allSecrets)

	// List models.
	allModels := []*summarizerv1pb.InferenceModel{}
	modelsPage, err := sclt.ListInferenceModels(ctx, &summarizerv1pb.ListInferenceModelsRequest{
		PageSize: 2,
	})
	require.NoError(t, err)
	assert.Len(t, modelsPage.Models, 2)
	allModels = append(allModels, modelsPage.Models...)

	modelsPage, err = sclt.ListInferenceModels(ctx, &summarizerv1pb.ListInferenceModelsRequest{
		PageSize:  2,
		PageToken: modelsPage.NextPageToken,
	})
	require.NoError(t, err)
	assert.Len(t, modelsPage.Models, 1)
	allModels = append(allModels, modelsPage.Models...)

	assertResourceListEqualsIgnoringOrder(t, expectedModels, allModels)

	// List policies.
	allPolicies := []*summarizerv1pb.InferencePolicy{}
	policiesPage, err := sclt.ListInferencePolicies(ctx, &summarizerv1pb.ListInferencePoliciesRequest{
		PageSize: 2,
	})
	require.NoError(t, err)
	assert.Len(t, policiesPage.Policies, 2)
	allPolicies = append(allPolicies, policiesPage.Policies...)

	policiesPage, err = sclt.ListInferencePolicies(ctx, &summarizerv1pb.ListInferencePoliciesRequest{
		PageSize:  2,
		PageToken: policiesPage.NextPageToken,
	})
	require.NoError(t, err)
	assert.Len(t, policiesPage.Policies, 1)
	allPolicies = append(allPolicies, policiesPage.Policies...)

	assertResourceListEqualsIgnoringOrder(t, expectedPolicies, allPolicies)
}

func TestService_RBAC(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	srv := newTestTLSServer(t)
	admin := createTestUser(t, srv, "admin")

	// Set up resources.
	clt, err := srv.NewClient(authtest.TestUser(admin.GetName()))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()
	// Resources that are created upfront.
	secret, model, policy := newTestResources(t, "1")
	createdSecret, err := sclt.CreateInferenceSecret(ctx, &summarizerv1pb.CreateInferenceSecretRequest{
		Secret: secret,
	})
	require.NoError(t, err)
	createdModel, err := sclt.CreateInferenceModel(ctx, &summarizerv1pb.CreateInferenceModelRequest{
		Model: model,
	})
	require.NoError(t, err)
	createdPolicy, err := sclt.CreateInferencePolicy(ctx, &summarizerv1pb.CreateInferencePolicyRequest{
		Policy: policy,
	})
	require.NoError(t, err)
	// Resources for further creation attempts.
	secret2, model2, policy2 := newTestResources(t, "2")

	cases := []struct {
		name string
		// resource is the resource kind whose RBAC is tested.
		resource string
		// verbs are the verbs that should be required to access given resource. To
		// make sure that all of them are required, we test each verb being denied
		// separately.
		verbs []string
		// fn is the function that tries to access the resource.
		fn func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error
	}{
		{
			name:     "create secret",
			resource: types.KindInferenceSecret,
			verbs:    []string{types.VerbCreate},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.CreateInferenceSecret(ctx, &summarizerv1pb.CreateInferenceSecretRequest{
					Secret: secret2,
				})
				return err
			},
		},
		{
			name:     "create model",
			resource: types.KindInferenceModel,
			verbs:    []string{types.VerbCreate},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.CreateInferenceModel(ctx, &summarizerv1pb.CreateInferenceModelRequest{
					Model: model2,
				})
				return err
			},
		},
		{
			name:     "create policy",
			resource: types.KindInferencePolicy,
			verbs:    []string{types.VerbCreate},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.CreateInferencePolicy(ctx, &summarizerv1pb.CreateInferencePolicyRequest{
					Policy: policy2,
				})
				return err
			},
		},
		{
			name:     "get secret",
			resource: types.KindInferenceSecret,
			verbs:    []string{types.VerbRead},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.GetInferenceSecret(ctx, &summarizerv1pb.GetInferenceSecretRequest{
					Name: "secret1",
				})
				return err
			},
		},
		{
			name:     "get model",
			resource: types.KindInferenceModel,
			verbs:    []string{types.VerbRead},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.GetInferenceModel(ctx, &summarizerv1pb.GetInferenceModelRequest{
					Name: "model1",
				})
				return err
			},
		},
		{
			name:     "get policy",
			resource: types.KindInferencePolicy,
			verbs:    []string{types.VerbRead},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.GetInferencePolicy(ctx, &summarizerv1pb.GetInferencePolicyRequest{
					Name: "policy1",
				})
				return err
			},
		},
		{
			name:     "update secret",
			resource: types.KindInferenceSecret,
			verbs:    []string{types.VerbUpdate},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.UpdateInferenceSecret(ctx, &summarizerv1pb.UpdateInferenceSecretRequest{
					Secret: createdSecret.Secret,
				})
				return err
			},
		},
		{
			name:     "update model",
			resource: types.KindInferenceModel,
			verbs:    []string{types.VerbUpdate},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.UpdateInferenceModel(ctx, &summarizerv1pb.UpdateInferenceModelRequest{
					Model: createdModel.Model,
				})
				return err
			},
		},
		{
			name:     "update policy",
			resource: types.KindInferencePolicy,
			verbs:    []string{types.VerbUpdate},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.UpdateInferencePolicy(ctx, &summarizerv1pb.UpdateInferencePolicyRequest{
					Policy: createdPolicy.Policy,
				})
				return err
			},
		},
		{
			name:     "upsert secret",
			resource: types.KindInferenceSecret,
			verbs:    []string{types.VerbCreate, types.VerbUpdate},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.UpsertInferenceSecret(ctx, &summarizerv1pb.UpsertInferenceSecretRequest{
					Secret: createdSecret.Secret,
				})
				return err
			},
		},
		{
			name:     "upsert model",
			resource: types.KindInferenceModel,
			verbs:    []string{types.VerbCreate, types.VerbUpdate},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.UpsertInferenceModel(ctx, &summarizerv1pb.UpsertInferenceModelRequest{
					Model: createdModel.Model,
				})
				return err
			},
		},
		{
			name:     "upsert policy",
			resource: types.KindInferencePolicy,
			verbs:    []string{types.VerbCreate, types.VerbUpdate},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.UpsertInferencePolicy(ctx, &summarizerv1pb.UpsertInferencePolicyRequest{
					Policy: createdPolicy.Policy,
				})
				return err
			},
		},
		{
			name:     "delete secret",
			resource: types.KindInferenceSecret,
			verbs:    []string{types.VerbDelete},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.DeleteInferenceSecret(ctx, &summarizerv1pb.DeleteInferenceSecretRequest{
					Name: "secret1",
				})
				return err
			},
		},
		{
			name:     "delete model",
			resource: types.KindInferenceModel,
			verbs:    []string{types.VerbDelete},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.DeleteInferenceModel(ctx, &summarizerv1pb.DeleteInferenceModelRequest{
					Name: "model1",
				})
				return err
			},
		},
		{
			name:     "delete policy",
			resource: types.KindInferencePolicy,
			verbs:    []string{types.VerbDelete},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.DeleteInferencePolicy(ctx, &summarizerv1pb.DeleteInferencePolicyRequest{
					Name: "policy1",
				})
				return err
			},
		},
	}

	for i, tc := range cases {
		for j, verb := range tc.verbs {
			t.Run(fmt.Sprintf("%s (%s)", tc.name, verb), func(t *testing.T) {
				user := createTestUser(
					t, srv, fmt.Sprintf("user-%d-%d", i+1, j+1),
					authtest.WithUserMutator(func(user types.User) {
						roleName := "deny-" + user.GetName()
						role, err := types.NewRole(roleName, types.RoleSpecV6{
							Deny: types.RoleConditions{
								Rules: []types.Rule{
									types.NewRule(tc.resource, []string{verb}),
								},
							},
						})
						require.NoError(t, err)
						_, err = srv.Auth().UpsertRole(ctx, role)
						require.NoError(t, err)
						user.AddRole(roleName)
					}),
				)

				clt, err := srv.NewClient(authtest.TestUser(user.GetName()))
				require.NoError(t, err)
				sclt := clt.SummarizerServiceClient()
				err = tc.fn(ctx, sclt)
				require.Error(t, err)
				assert.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, got %v", err)
			})
		}
	}
}

func newSessionEndEvent() *apievents.SessionEnd {
	startTime := time.Date(2020, 3, 30, 15, 58, 54, 561*int(time.Millisecond), time.UTC)
	endTime := startTime.Add(time.Minute)
	return &apievents.SessionEnd{
		Metadata: apievents.Metadata{
			Index: 20,
			Type:  events.SessionEndEvent,
			ID:    "da455e0f-c27d-459f-a218-4e83b3db9426",
			Code:  events.SessionEndCode,
			Time:  endTime,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerVersion:   teleport.Version,
			ServerID:        "6a7c593d-345a-431f-9e21-4049be982fa5",
			ServerNamespace: "default",
			ServerLabels:    map[string]string{"env": "prod"},
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: "cb116fb0-9227-4889-9392-aedd13a914a6",
		},
		UserMetadata: apievents.UserMetadata{
			User: "alice",
		},
		EnhancedRecording: true,
		Interactive:       true,
		Participants:      []string{"alice", "bob"},
		StartTime:         startTime,
		EndTime:           endTime,
	}
}

func newTestSummary(t *testing.T, sessionEnd *apievents.SessionEnd) *summarizerv1pb.Summary {
	t.Helper()

	inferenceStartTime := sessionEnd.EndTime.Add(time.Minute)
	inferenceEndTime := inferenceStartTime.Add(time.Minute)
	endEventFields, err := events.ToEventFields(sessionEnd)
	require.NoError(t, err)
	endEventStruct, err := structpb.NewStruct(endEventFields)
	require.NoError(t, err)

	return &summarizerv1pb.Summary{
		SessionId:           sessionEnd.SessionID,
		State:               summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS,
		InferenceStartedAt:  timestamppb.New(inferenceStartTime),
		InferenceFinishedAt: timestamppb.New(inferenceEndTime),
		Content:             "This is a test summary content.",
		ModelName:           "some-model",
		SessionEndEvent:     endEventStruct,
	}
}

func TestService_GetSummary(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	srv := newTestTLSServer(t)
	user := createTestUser(t, srv, "alice")

	clt, err := srv.NewClient(authtest.TestUser(user.GetName()))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()

	sessionEnd := newSessionEndEvent()
	expectedSummary := newTestSummary(t, sessionEnd)
	b, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(expectedSummary)
	require.NoError(t, err)
	srv.AuthServer.AuthServer.UploadSummary(ctx, session.ID(expectedSummary.SessionId), bytes.NewReader(b))

	// Test fetching an existing summary.
	got, err := sclt.GetSummary(ctx, &summarizerv1pb.GetSummaryRequest{
		SessionId: expectedSummary.SessionId,
	})
	require.NoError(t, err)
	assert.Empty(t, cmp.Diff(expectedSummary, got.Summary, protocmp.Transform()))

	// Make sure that the session end event can be fully recovered from the
	// unstructured representation.
	gotSessionEnd, err := events.FromEventFields(got.Summary.SessionEndEvent.AsMap())
	require.NoError(t, err)
	assert.Empty(t, cmp.Diff(sessionEnd, gotSessionEnd, protocmp.Transform()))

	// Test fetching a summary that doesn't exist.
	_, err = sclt.GetSummary(ctx, &summarizerv1pb.GetSummaryRequest{
		SessionId: "aa6bd352-7d90-4802-927e-9295872f37ad",
	})
	require.Error(t, err)
	assert.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)
}

func TestService_GetSummary_RBAC(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	srv := newTestTLSServer(t)

	// Create session summaries.
	// Session of Alice and Bob
	sessionEnd1 := newSessionEndEvent()
	summary1 := newTestSummary(t, sessionEnd1)
	b, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(summary1)
	require.NoError(t, err)
	srv.AuthServer.AuthServer.UploadSummary(ctx, session.ID(summary1.SessionId), bytes.NewReader(b))

	// Session of Bob and Mary
	sessionEnd2 := newSessionEndEvent()
	sessionEnd2.SessionID = "0fd10888-a48d-45b7-9a10-dc4321f7b159"
	sessionEnd2.UserMetadata.User = "bob"
	sessionEnd2.Participants = []string{"bob", "mary"}
	summary2 := newTestSummary(t, sessionEnd2)
	b, err = protojson.MarshalOptions{UseProtoNames: true}.Marshal(summary2)
	require.NoError(t, err)
	srv.AuthServer.AuthServer.UploadSummary(ctx, session.ID(summary2.SessionId), bytes.NewReader(b))

	// Add Alice.
	createTestUser(t, srv, "alice")
	aliceClt, err := srv.NewClient(authtest.TestUser("alice"))
	require.NoError(t, err)
	aliceSclt := aliceClt.SummarizerServiceClient()

	// Alice should only see the first session (see the "where" condition in
	// user's role).
	_, err = aliceSclt.GetSummary(ctx, &summarizerv1pb.GetSummaryRequest{
		SessionId: summary1.SessionId,
	})
	require.NoError(t, err)
	_, err = aliceSclt.GetSummary(ctx, &summarizerv1pb.GetSummaryRequest{
		SessionId: summary2.SessionId,
	})
	require.Error(t, err)
	assert.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, got %v", err)

	// Add Bob.
	createTestUser(t, srv, "bob")
	bobClt, err := srv.NewClient(authtest.TestUser("bob"))
	require.NoError(t, err)
	bobSclt := bobClt.SummarizerServiceClient()

	// Bob should be able to see both sessions, as he participated in both.
	_, err = bobSclt.GetSummary(ctx, &summarizerv1pb.GetSummaryRequest{
		SessionId: summary1.SessionId,
	})
	require.NoError(t, err)
	_, err = bobSclt.GetSummary(ctx, &summarizerv1pb.GetSummaryRequest{
		SessionId: summary2.SessionId,
	})
	require.NoError(t, err)

	// Add Mary.
	createTestUser(t, srv, "mary")
	maryClt, err := srv.NewClient(authtest.TestUser("mary"))
	require.NoError(t, err)
	marySclt := maryClt.SummarizerServiceClient()

	// Mary should only see the second session (see the "where" condition in
	// user's role).
	_, err = marySclt.GetSummary(ctx, &summarizerv1pb.GetSummaryRequest{
		SessionId: summary1.SessionId,
	})
	require.Error(t, err)
	assert.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, got %v", err)
	_, err = marySclt.GetSummary(ctx, &summarizerv1pb.GetSummaryRequest{
		SessionId: summary2.SessionId,
	})
	require.NoError(t, err)

	// Add an account that doesn't have any access to session recordings (there's
	// a special case for that in the code).
	_, _, err = authtest.CreateUserAndRole(srv.Auth(), "intern", []string{}, []types.Rule{})
	require.NoError(t, err)
	internClt, err := srv.NewClient(authtest.TestUser("intern"))
	require.NoError(t, err)
	internSclt := internClt.SummarizerServiceClient()

	_, err = internSclt.GetSummary(ctx, &summarizerv1pb.GetSummaryRequest{
		SessionId: summary1.SessionId,
	})
	require.Error(t, err)
	assert.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, got %v", err)
}
