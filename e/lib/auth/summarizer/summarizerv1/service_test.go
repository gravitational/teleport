package summarizerv1

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
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
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/session"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

func TestMain(m *testing.M) {
	modules.SetModules(&modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Policy: {Enabled: true},
			},
		},
	})
	os.Exit(m.Run())
}

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
		Emitter:           authServer.AuthServer.GetEmitter(),
		Decrypter:         &fakeEncryptedIO{},
		UsageReporter:     authServer.AuthServer.UsageReporter,
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

type fakeUsageReporter struct {
	events []usagereporter.Anonymizable
}

func (f *fakeUsageReporter) AnonymizeAndSubmit(event ...usagereporter.Anonymizable) {
	f.events = append(f.events, event...)
}

type newTestTLSServerOptions struct {
	usageReporter usagereporter.UsageReporter
	emitter       apievents.Emitter
}

type newTestTLSServerOption func(*newTestTLSServerOptions)

func withUsageReporter(ur usagereporter.UsageReporter) newTestTLSServerOption {
	return func(o *newTestTLSServerOptions) {
		o.usageReporter = ur
	}
}

func withEmitter(emitter apievents.Emitter) newTestTLSServerOption {
	return func(o *newTestTLSServerOptions) {
		o.emitter = emitter
	}
}

func newTestTLSServer(t testing.TB, opts ...newTestTLSServerOption) *authtest.TLSServer {
	opt := &newTestTLSServerOptions{}
	for _, o := range opts {
		o(opt)
	}
	as, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir: t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, as.Close()) })

	if opt.usageReporter != nil {
		as.AuthServer.SetUsageReporter(opt.usageReporter)
	}

	if opt.emitter != nil {
		as.AuthServer.SetEmitter(opt.emitter)
	}

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

func newBedrockModel(name string) *summarizerv1pb.InferenceModel {
	return summarizer.NewInferenceModel(name, &summarizerv1pb.InferenceModelSpec{
		Provider: &summarizerv1pb.InferenceModelSpec_Bedrock{
			Bedrock: &summarizerv1pb.BedrockProvider{
				Region:         "us-west-2",
				BedrockModelId: "anthropic.claude-3-5-sonnet-20240620-v1:0",
			},
		},
	})
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

	// Test a valid Bedrock model. This one should be accepted.
	validBedrockModel := newBedrockModel("valid-bedrock-model")
	createdModel, err = sclt.CreateInferenceModel(ctx, &summarizerv1pb.CreateInferenceModelRequest{
		Model: validBedrockModel,
	})
	require.NoError(t, err)
	assertResourceEquals(t, validBedrockModel, createdModel.Model)

	gotModel, err = sclt.GetInferenceModel(ctx, &summarizerv1pb.GetInferenceModelRequest{
		Name: "valid-bedrock-model",
	})
	require.NoError(t, err)
	assertResourceEquals(t, validBedrockModel, gotModel.Model)

	// Test a Bedrock model that has a reserved name. This one should be
	// rejected.
	invalidBedrockModel := newBedrockModel(summarizer.CloudDefaultInferenceModelName)
	createdModel, err = sclt.CreateInferenceModel(ctx, &summarizerv1pb.CreateInferenceModelRequest{
		Model: invalidBedrockModel,
	})
	assert.ErrorIs(t, err, &trace.BadParameterError{
		Message: `metadata.name "teleport-cloud-default" is reserved`,
	})
	assert.Nil(t, createdModel)

	gotModel, err = sclt.GetInferenceModel(ctx, &summarizerv1pb.GetInferenceModelRequest{
		Name: summarizer.CloudDefaultInferenceModelName,
	})
	assert.ErrorAs(t, err, new(*trace.NotFoundError))
	assert.Nil(t, gotModel)
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
	cloudDefaultModel := newBedrockModel("teleport-cloud-default")

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

	// Create the cloud default model directly in the backend, as it can't be
	// created via an API.
	_, err = srv.AuthServer.AuthServer.Summarizer.CreateInferenceModel(ctx, cloudDefaultModel)
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

	// Test attempting to update the cloud default model (should fail).
	modifiedCloudDefaultModel := proto.CloneOf(cloudDefaultModel)
	modifiedCloudDefaultModel.Spec.GetBedrock().Temperature = 0.2
	_, err = sclt.UpdateInferenceModel(ctx, &summarizerv1pb.UpdateInferenceModelRequest{
		Model: modifiedCloudDefaultModel,
	})
	assert.ErrorAs(t, err, new(*trace.BadParameterError))

	gotModel, err = sclt.GetInferenceModel(ctx, &summarizerv1pb.GetInferenceModelRequest{
		Name: "teleport-cloud-default",
	})
	require.NoError(t, err)
	assertResourceEquals(t, cloudDefaultModel, gotModel.Model)

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
	cloudDefaultModel := newBedrockModel("teleport-cloud-default")

	// Create the cloud default model directly in the backend, as it can't be
	// created via an API.
	_, err = srv.AuthServer.AuthServer.Summarizer.CreateInferenceModel(ctx, cloudDefaultModel)
	require.NoError(t, err)

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

	// Test attempting to update the cloud default model (should fail).
	modifiedCloudDefaultModel := proto.CloneOf(cloudDefaultModel)
	modifiedCloudDefaultModel.Spec.GetBedrock().Temperature = 0.2
	_, err = sclt.UpsertInferenceModel(ctx, &summarizerv1pb.UpsertInferenceModelRequest{
		Model: modifiedCloudDefaultModel,
	})
	assert.ErrorAs(t, err, new(*trace.BadParameterError))

	gotModel, err = sclt.GetInferenceModel(ctx, &summarizerv1pb.GetInferenceModelRequest{
		Name: "teleport-cloud-default",
	})
	require.NoError(t, err)
	assertResourceEquals(t, cloudDefaultModel, gotModel.Model)

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
	cloudDefaultModel := newBedrockModel("teleport-cloud-default")

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

	// Create the cloud default model directly in the backend, as it can't be
	// created via an API.
	_, err = srv.AuthServer.AuthServer.Summarizer.CreateInferenceModel(ctx, cloudDefaultModel)
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

	// Test attempting to delete the cloud default model (should fail).
	_, err = sclt.DeleteInferenceModel(ctx, &summarizerv1pb.DeleteInferenceModelRequest{
		Name: "teleport-cloud-default",
	})
	assert.ErrorAs(t, err, new(*trace.BadParameterError))

	gotModel, err := sclt.GetInferenceModel(ctx, &summarizerv1pb.GetInferenceModelRequest{
		Name: "teleport-cloud-default",
	})
	require.NoError(t, err)
	assertResourceEquals(t, cloudDefaultModel, gotModel.Model)
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
			ID:    uuid.NewString(),
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
			SessionID: uuid.NewString(),
		},
		UserMetadata: apievents.UserMetadata{
			User: "alice",
		},
		ConnectionMetadata: apievents.ConnectionMetadata{
			Protocol: apievents.EventProtocolSSH,
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
	usageReporter := &fakeUsageReporter{}
	srv := newTestTLSServer(t, withUsageReporter(usageReporter))
	user := createTestUser(t, srv, "alice")

	clt, err := srv.NewClient(authtest.TestUser(user.GetName()))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()

	t.Run("unencrypted summary", func(t *testing.T) {
		// Summary 1 is in a pending state.
		session1End := newSessionEndEvent()
		summary1 := newTestSummary(t, session1End)
		summary1.Content = ""
		summary1.InferenceFinishedAt = nil
		summary1.State = summarizerv1pb.SummaryState_SUMMARY_STATE_PENDING
		b, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(summary1)
		require.NoError(t, err)
		srv.AuthServer.AuthServer.UploadPendingSummary(ctx, session.ID(summary1.SessionId), bytes.NewReader(b))

		// Summary 2 is in a final state.
		session2End := newSessionEndEvent()
		summary2 := newTestSummary(t, session2End)
		summary2Pending := proto.CloneOf(summary2)
		summary2Pending.Content = ""
		summary2Pending.InferenceFinishedAt = nil
		summary2Pending.State = summarizerv1pb.SummaryState_SUMMARY_STATE_PENDING

		// Upload the pending state of summary 2.
		b, err = protojson.MarshalOptions{UseProtoNames: true}.Marshal(summary2Pending)
		require.NoError(t, err)
		_, err = srv.AuthServer.AuthServer.UploadPendingSummary(ctx, session.ID(summary2Pending.SessionId), bytes.NewReader(b))
		require.NoError(t, err)

		// Upload the final state of summary 2.
		b, err = protojson.MarshalOptions{UseProtoNames: true}.Marshal(summary2)
		require.NoError(t, err)
		_, err = srv.AuthServer.AuthServer.UploadSummary(ctx, session.ID(summary2.SessionId), bytes.NewReader(b))
		require.NoError(t, err)

		// Test fetching a pending summary.
		got, err := sclt.GetSummary(ctx, &summarizerv1pb.GetSummaryRequest{
			SessionId: summary1.SessionId,
		})
		require.NoError(t, err)
		assert.Empty(t, cmp.Diff(summary1, got.Summary, protocmp.Transform()))

		// Test fetching a final summary.
		got, err = sclt.GetSummary(ctx, &summarizerv1pb.GetSummaryRequest{
			SessionId: summary2.SessionId,
		})
		require.NoError(t, err)
		assert.Empty(t, cmp.Diff(summary2, got.Summary, protocmp.Transform()))

		// Make sure that the session end event can be fully recovered from the
		// unstructured representation.
		gotSessionEnd, err := events.FromEventFields(got.Summary.SessionEndEvent.AsMap())
		require.NoError(t, err)
		assert.Empty(t, cmp.Diff(session2End, gotSessionEnd, protocmp.Transform()))

		// Test fetching a summary that doesn't exist.
		_, err = sclt.GetSummary(ctx, &summarizerv1pb.GetSummaryRequest{
			SessionId: uuid.NewString(),
		})
		require.Error(t, err)
		assert.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)
	})

	t.Run("encrypted summary", func(t *testing.T) {
		sessionEnd := newSessionEndEvent()
		// Use a different session ID to avoid conflicts with the previous test.
		sessionEnd.SessionID = "0fd10888-a48d-45b7-9a10-dc4321f7b159"
		expectedSummary := newTestSummary(t, sessionEnd)
		b, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(expectedSummary)
		require.NoError(t, err)

		srv.AuthServer.AuthServer.UploadSummary(ctx, session.ID(expectedSummary.SessionId), bytes.NewReader(encrypt(b)))
		// Test fetching an existing encrypted summary.
		got, err := sclt.GetSummary(ctx, &summarizerv1pb.GetSummaryRequest{
			SessionId: expectedSummary.SessionId,
		})
		require.NoError(t, err)
		assert.Empty(t, cmp.Diff(expectedSummary, got.Summary, protocmp.Transform()))
	})

	// Validate that usage events were emitted for both GetSummary calls.
	sessionSummaryAccessEvents := make([]*usagereporter.SessionSummaryAccessEvent, 0, 3)
	for _, event := range usageReporter.events {
		if summaryEvent, ok := event.(*usagereporter.SessionSummaryAccessEvent); ok {
			sessionSummaryAccessEvents = append(sessionSummaryAccessEvents, summaryEvent)
		}
	}

	require.Len(t, sessionSummaryAccessEvents, 3, "expected 3 usage events to be emitted")
	for i, event := range sessionSummaryAccessEvents {
		assert.Equal(t, "alice", event.UserName, "event %d: unexpected user name", i)
		assert.NotEmpty(t, event.SessionType, "event %d: session type should not be empty", i)
	}
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

	// Create a role that allows viewing all sessions, but only on nodes where
	// the role labels match.
	_, canViewUserRole, err := authtest.CreateUserAndRole(srv.Auth(), "can_view_user", []string{}, []types.Rule{
		{
			Resources: []string{types.KindSession},
			Verbs:     []string{types.VerbRead, types.VerbList},
			Where:     `can_view()`,
		},
	})
	require.NoError(t, err)
	canViewUser, err := srv.NewClient(authtest.TestUser("can_view_user"))
	require.NoError(t, err)

	_, err = canViewUser.SummarizerServiceClient().GetSummary(ctx, &summarizerv1pb.GetSummaryRequest{
		SessionId: summary1.SessionId,
	})
	require.NoError(t, err)

	// Remove the node_labels condition from the role, which should make the
	// user lose access to everything (because of the "can_view()" condition).
	canViewUserRole.SetNodeLabels(types.Allow, nil)
	_, err = srv.Auth().UpdateRole(ctx, canViewUserRole)
	require.NoError(t, err)

	_, err = canViewUser.SummarizerServiceClient().GetSummary(ctx, &summarizerv1pb.GetSummaryRequest{
		SessionId: summary1.SessionId,
	})
	require.Error(t, err)
	assert.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, got %v", err)
}

// encryptedIO is really just a reversible transform, so we fake encryption by encoding/decoding as hex
type fakeEncryptedIO struct{}

func encrypt(buf []byte) []byte {
	var writer bytes.Buffer
	writer.Write([]byte("age-encryption.org")) // fake header
	writer.WriteString(hex.EncodeToString(buf))

	return writer.Bytes()
}

const agePrefix = "age-encryption.org"

func (f *fakeEncryptedIO) WithDecryption(ctx context.Context, reader io.Reader) (io.Reader, error) {
	// read and discard fake header
	header := make([]byte, len(agePrefix))
	_, err := io.ReadFull(reader, header)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if string(header) != agePrefix {
		return nil, trace.BadParameter("invalid encryption header")
	}
	return hex.NewDecoder(reader), nil
}

func TestService_AuditEvents(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	emitter := &eventstest.MockRecorderEmitter{}
	srv := newTestTLSServer(t, withEmitter(emitter))
	user := createTestUser(t, srv, "test-user")

	clt, err := srv.NewClient(authtest.TestUser(user.GetName()))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()

	getRecentEvents := func(eventTypes ...string) []apievents.AuditEvent {
		eventTypeMap := make(map[string]bool)
		for _, et := range eventTypes {
			eventTypeMap[et] = true
		}

		var result []apievents.AuditEvent
		for _, evt := range emitter.Events() {
			if len(eventTypes) == 0 || eventTypeMap[evt.GetType()] {
				result = append(result, evt)
			}
		}
		return result
	}

	t.Run("InferenceModel events", func(t *testing.T) {
		secret, model, _ := newTestResources(t, "audit1")

		_, err := sclt.CreateInferenceSecret(ctx, &summarizerv1pb.CreateInferenceSecretRequest{
			Secret: secret,
		})
		require.NoError(t, err)

		createdModel, err := sclt.CreateInferenceModel(ctx, &summarizerv1pb.CreateInferenceModelRequest{
			Model: model,
		})
		require.NoError(t, err)

		evts := getRecentEvents(events.InferenceModelCreateEvent)
		require.NotEmpty(t, evts, "expected at least one model create event")
		createEvt, ok := evts[len(evts)-1].(*apievents.InferenceModelCreate)
		require.True(t, ok, "expected InferenceModelCreate event, got %T", evts[len(evts)-1])
		assert.Equal(t, events.InferenceModelCreateCode, createEvt.Code)
		assert.Equal(t, "modelaudit1", createEvt.ResourceMetadata.Name)
		assert.Equal(t, user.GetName(), createEvt.User)

		createdModel.Model.Spec.GetOpenai().Temperature = 1.5
		_, err = sclt.UpdateInferenceModel(ctx, &summarizerv1pb.UpdateInferenceModelRequest{
			Model: createdModel.Model,
		})
		require.NoError(t, err)

		evts = getRecentEvents(events.InferenceModelUpdateEvent)
		require.NotEmpty(t, evts, "expected at least one model update event")
		updateEvt, ok := evts[len(evts)-1].(*apievents.InferenceModelUpdate)
		require.True(t, ok, "expected InferenceModelUpdate event got %T", evts[len(evts)-1])
		assert.Equal(t, events.InferenceModelUpdateCode, updateEvt.Code)
		assert.Equal(t, "modelaudit1", updateEvt.ResourceMetadata.Name)
		assert.Equal(t, user.GetName(), updateEvt.User)

		_, err = sclt.DeleteInferenceModel(ctx, &summarizerv1pb.DeleteInferenceModelRequest{
			Name: "modelaudit1",
		})
		require.NoError(t, err)

		evts = getRecentEvents(events.InferenceModelDeleteEvent)
		require.NotEmpty(t, evts, "expected at least one model delete event")
		deleteEvt, ok := evts[len(evts)-1].(*apievents.InferenceModelDelete)
		require.True(t, ok, "expected InferenceModelDelete event got %T", evts[len(evts)-1])
		assert.Equal(t, events.InferenceModelDeleteCode, deleteEvt.Code)
		assert.Equal(t, "modelaudit1", deleteEvt.ResourceMetadata.Name)
		assert.Equal(t, user.GetName(), deleteEvt.User)
	})

	t.Run("InferenceSecret events", func(t *testing.T) {
		secret, _, _ := newTestResources(t, "audit2")

		_, err := sclt.CreateInferenceSecret(ctx, &summarizerv1pb.CreateInferenceSecretRequest{
			Secret: secret,
		})
		require.NoError(t, err)

		evts := getRecentEvents(events.InferenceSecretCreateEvent)
		require.NotEmpty(t, evts, "expected at least one secret create event")
		createEvt, ok := evts[len(evts)-1].(*apievents.InferenceSecretCreate)
		require.True(t, ok, "expected InferenceSecretCreate event, got %T", evts[len(evts)-1])
		assert.Equal(t, events.InferenceSecretCreateCode, createEvt.Code)
		assert.Equal(t, "secretaudit2", createEvt.ResourceMetadata.Name)
		assert.Equal(t, user.GetName(), createEvt.User)

		backendSecret, err := srv.AuthServer.AuthServer.GetInferenceSecret(ctx, "secretaudit2")
		require.NoError(t, err)
		backendSecret.Spec.Value = "updated-value"
		_, err = sclt.UpdateInferenceSecret(ctx, &summarizerv1pb.UpdateInferenceSecretRequest{
			Secret: backendSecret,
		})
		require.NoError(t, err)

		evts = getRecentEvents(events.InferenceSecretUpdateEvent)
		require.NotEmpty(t, evts, "expected at least one secret update event")
		updateEvt, ok := evts[len(evts)-1].(*apievents.InferenceSecretUpdate)
		require.True(t, ok, "expected InferenceSecretUpdate event, got %T", evts[len(evts)-1])
		assert.Equal(t, events.InferenceSecretUpdateCode, updateEvt.Code)
		assert.Equal(t, "secretaudit2", updateEvt.ResourceMetadata.Name)
		assert.Equal(t, user.GetName(), updateEvt.User)

		_, err = sclt.DeleteInferenceSecret(ctx, &summarizerv1pb.DeleteInferenceSecretRequest{
			Name: "secretaudit2",
		})
		require.NoError(t, err)

		evts = getRecentEvents(events.InferenceSecretDeleteEvent)
		require.NotEmpty(t, evts, "expected at least one secret delete event")
		deleteEvt, ok := evts[len(evts)-1].(*apievents.InferenceSecretDelete)
		require.True(t, ok, "expected InferenceSecretDelete event, got %T", evts[len(evts)-1])
		assert.Equal(t, events.InferenceSecretDeleteCode, deleteEvt.Code)
		assert.Equal(t, "secretaudit2", deleteEvt.ResourceMetadata.Name)
		assert.Equal(t, user.GetName(), deleteEvt.User)
	})

	t.Run("InferencePolicy events", func(t *testing.T) {
		secret, model, policy := newTestResources(t, "audit3")

		_, err := sclt.CreateInferenceSecret(ctx, &summarizerv1pb.CreateInferenceSecretRequest{
			Secret: secret,
		})
		require.NoError(t, err)
		_, err = sclt.CreateInferenceModel(ctx, &summarizerv1pb.CreateInferenceModelRequest{
			Model: model,
		})
		require.NoError(t, err)

		createdPolicy, err := sclt.CreateInferencePolicy(ctx, &summarizerv1pb.CreateInferencePolicyRequest{
			Policy: policy,
		})
		require.NoError(t, err)

		evts := getRecentEvents(events.InferencePolicyCreateEvent)
		require.NotEmpty(t, evts, "expected at least one policy create event")
		createEvt, ok := evts[len(evts)-1].(*apievents.InferencePolicyCreate)
		require.True(t, ok, "expected InferencePolicyCreate event, got %T", evts[len(evts)-1])
		assert.Equal(t, events.InferencePolicyCreateCode, createEvt.Code)
		assert.Equal(t, "policyaudit3", createEvt.ResourceMetadata.Name)
		assert.Equal(t, user.GetName(), createEvt.User)

		createdPolicy.Policy.Spec.Filter = `equals(resource.metadata.labels["env"], "dev")`
		_, err = sclt.UpdateInferencePolicy(ctx, &summarizerv1pb.UpdateInferencePolicyRequest{
			Policy: createdPolicy.Policy,
		})
		require.NoError(t, err)

		evts = getRecentEvents(events.InferencePolicyUpdateEvent)
		require.NotEmpty(t, evts, "expected at least one policy update event")
		updateEvt, ok := evts[len(evts)-1].(*apievents.InferencePolicyUpdate)
		require.True(t, ok, "expected InferencePolicyUpdate event, got %T", evts[len(evts)-1])
		assert.Equal(t, events.InferencePolicyUpdateCode, updateEvt.Code)
		assert.Equal(t, "policyaudit3", updateEvt.ResourceMetadata.Name)
		assert.Equal(t, user.GetName(), updateEvt.User)

		_, err = sclt.DeleteInferencePolicy(ctx, &summarizerv1pb.DeleteInferencePolicyRequest{
			Name: "policyaudit3",
		})
		require.NoError(t, err)

		evts = getRecentEvents(events.InferencePolicyDeleteEvent)
		require.NotEmpty(t, evts, "expected at least one policy delete event")
		deleteEvt, ok := evts[len(evts)-1].(*apievents.InferencePolicyDelete)
		require.True(t, ok, "expected InferencePolicyDelete event, got %T", evts[len(evts)-1])
		assert.Equal(t, events.InferencePolicyDeleteCode, deleteEvt.Code)
		assert.Equal(t, "policyaudit3", deleteEvt.ResourceMetadata.Name)
		assert.Equal(t, user.GetName(), deleteEvt.User)
	})
}

func TestService_TestInferenceModel(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	srv := newTestTLSServer(t)
	user := createTestUser(t, srv, "test-user")
	mockOpenAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": {"message": "Invalid API key"}}`))
	}))
	t.Cleanup(mockOpenAI.Close)

	clt, err := srv.NewClient(authtest.TestUser(user.GetName()))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()

	tests := []struct {
		name            string
		req             *summarizerv1pb.TestInferenceModelRequest
		expectError     bool
		expectSuccess   bool
		errorContains   string
		messageContains string
	}{
		{
			name: "missing model spec",
			req: &summarizerv1pb.TestInferenceModelRequest{
				Model: nil,
			},
			expectSuccess:   false,
			messageContains: "model spec is required",
		},
		{
			name: "OpenAI without secret",
			req: &summarizerv1pb.TestInferenceModelRequest{
				Model: &summarizerv1pb.InferenceModelSpec{
					Provider: &summarizerv1pb.InferenceModelSpec_Openai{
						Openai: &summarizerv1pb.OpenAIProvider{
							OpenaiModelId: "gpt-4o",
						},
					},
				},
			},
			expectSuccess:   false,
			messageContains: "api_key_secret_ref is required for OpenAI models when no secret is provided in the request",
		},
		{
			name: "OpenAI with invalid API key",
			req: &summarizerv1pb.TestInferenceModelRequest{
				Model: &summarizerv1pb.InferenceModelSpec{
					Provider: &summarizerv1pb.InferenceModelSpec_Openai{
						Openai: &summarizerv1pb.OpenAIProvider{
							OpenaiModelId: "gpt-4o",
							BaseUrl:       mockOpenAI.URL,
						},
					},
				},
				Secret: &summarizerv1pb.InferenceSecretSpec{
					Value: "test-api-key",
				},
			},
			expectSuccess: false,
			// When no client factory is configured, OpenAI provider uses default client
			// which makes a real API call to the mock server, so we expect an authentication error
			messageContains: "Invalid API key",
		},
		{
			name: "unsupported provider type",
			req: &summarizerv1pb.TestInferenceModelRequest{
				Model: &summarizerv1pb.InferenceModelSpec{
					Provider: nil,
				},
			},
			expectSuccess:   false,
			messageContains: "invalid model spec: missing or unsupported inference provider in spec, supported providers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := sclt.TestInferenceModel(ctx, tt.req)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, resp)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, tt.expectSuccess, resp.Success)
			if tt.messageContains != "" {
				assert.Contains(t, resp.Message, tt.messageContains)
			}
		})
	}
}

func TestService_IsEnabled(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	proxyClientCreator := func(t *testing.T, srv *authtest.TLSServer) summarizerv1pb.SummarizerServiceClient {
		proxyClt, err := srv.NewClient(authtest.TestBuiltin(types.RoleProxy))
		require.NoError(t, err)
		t.Cleanup(func() { proxyClt.Close() })
		return proxyClt.SummarizerServiceClient()
	}

	tests := []struct {
		name             string
		setup            func(*testing.T, *authtest.TLSServer)
		createClient     func(*testing.T, *authtest.TLSServer) summarizerv1pb.SummarizerServiceClient
		errAssertionFunc require.ErrorAssertionFunc
		expectResult     bool
	}{
		{
			name: "not authorized - non-proxy client",
			createClient: func(t *testing.T, srv *authtest.TLSServer) summarizerv1pb.SummarizerServiceClient {
				user := createTestUser(t, srv, "test-user")
				userClt, err := srv.NewClient(authtest.TestUser(user.GetName()))
				require.NoError(t, err)
				t.Cleanup(func() { userClt.Close() })
				return userClt.SummarizerServiceClient()
			},
			setup: func(_ *testing.T, _ *authtest.TLSServer) {},
			errAssertionFunc: func(t require.TestingT, err error, i ...any) {
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, got %v", err)
			},
		},
		{
			name:             "disabled when no models or policies exist",
			setup:            func(_ *testing.T, _ *authtest.TLSServer) {},
			createClient:     proxyClientCreator,
			errAssertionFunc: require.NoError,
			expectResult:     false,
		},
		{
			name: "disabled when only models exist",
			setup: func(t *testing.T, srv *authtest.TLSServer) {
				secret, model, _ := newTestResources(t, "isenabled1")
				_, err := srv.AuthServer.AuthServer.CreateInferenceSecret(ctx, secret)
				require.NoError(t, err)
				_, err = srv.AuthServer.AuthServer.CreateInferenceModel(ctx, model)
				require.NoError(t, err)
			},
			createClient:     proxyClientCreator,
			errAssertionFunc: require.NoError,
			expectResult:     false,
		},
		{
			name: "disabled when only policies exist",
			setup: func(t *testing.T, srv *authtest.TLSServer) {
				secret, model, policy := newTestResources(t, "isenabled2")
				_, err := srv.AuthServer.AuthServer.CreateInferenceSecret(ctx, secret)
				require.NoError(t, err)
				_, err = srv.AuthServer.AuthServer.CreateInferenceModel(ctx, model)
				require.NoError(t, err)
				_, err = srv.AuthServer.AuthServer.CreateInferencePolicy(ctx, policy)
				require.NoError(t, err)

				// delete the model so only the policy remains
				err = srv.AuthServer.AuthServer.DeleteInferenceModel(ctx, model.GetMetadata().GetName())
				require.NoError(t, err)
			},
			createClient:     proxyClientCreator,
			errAssertionFunc: require.NoError,
			expectResult:     false,
		},
		{
			name: "enabled when both models and policies exist",
			setup: func(t *testing.T, srv *authtest.TLSServer) {
				secret, model, policy := newTestResources(t, "isenabled3")
				_, err := srv.AuthServer.AuthServer.CreateInferenceSecret(ctx, secret)
				require.NoError(t, err)
				_, err = srv.AuthServer.AuthServer.CreateInferenceModel(ctx, model)
				require.NoError(t, err)
				_, err = srv.AuthServer.AuthServer.CreateInferencePolicy(ctx, policy)
				require.NoError(t, err)
			},
			createClient:     proxyClientCreator,
			errAssertionFunc: require.NoError,
			expectResult:     true,
		},
		{
			name: "enabled with multiple models and policies",
			setup: func(t *testing.T, srv *authtest.TLSServer) {
				secret1, model1, policy1 := newTestResources(t, "isenabled4")
				secret2, model2, policy2 := newTestResources(t, "isenabled5")

				_, err := srv.AuthServer.AuthServer.CreateInferenceSecret(ctx, secret1)
				require.NoError(t, err)
				_, err = srv.AuthServer.AuthServer.CreateInferenceModel(ctx, model1)
				require.NoError(t, err)
				_, err = srv.AuthServer.AuthServer.CreateInferencePolicy(ctx, policy1)
				require.NoError(t, err)

				_, err = srv.AuthServer.AuthServer.CreateInferenceSecret(ctx, secret2)
				require.NoError(t, err)
				_, err = srv.AuthServer.AuthServer.CreateInferenceModel(ctx, model2)
				require.NoError(t, err)
				_, err = srv.AuthServer.AuthServer.CreateInferencePolicy(ctx, policy2)
				require.NoError(t, err)
			},
			createClient:     proxyClientCreator,
			errAssertionFunc: require.NoError,
			expectResult:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := newTestTLSServer(t)
			tt.setup(t, srv)
			resp, err := tt.createClient(t, srv).IsEnabled(ctx, &summarizerv1pb.IsEnabledRequest{})
			tt.errAssertionFunc(t, err)
			assert.Equal(t, tt.expectResult, resp.GetEnabled())
		})
	}
}
