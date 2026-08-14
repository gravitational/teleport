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
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

type testPlugin struct {
	unrestrictedBedrock bool
}

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
		Authorizer:                       authServer.Authorizer,
		Cache:                            authServer.AuthServer,
		Backend:                          authServer.AuthServer,
		SummaryDownloader:                authServer.AuthServer,
		Emitter:                          authServer.AuthServer.GetEmitter(),
		Decrypter:                        &fakeEncryptedIO{},
		UsageReporter:                    authServer.AuthServer.UsageReporter,
		IsLicensed:                       func() bool { return true },
		EnableBedrockWithoutRestrictions: p.unrestrictedBedrock,
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
	usageReporter       usagereporter.UsageReporter
	emitter             apievents.Emitter
	unrestrictedBedrock bool
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

func withUnrestrictedBedrock() newTestTLSServerOption {
	return func(o *newTestTLSServerOptions) {
		o.unrestrictedBedrock = true
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
		err = cfg.APIConfig.PluginRegistry.Add(&testPlugin{
			unrestrictedBedrock: opt.unrestrictedBedrock,
		})
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
				Resources: []string{types.KindInferenceSecret, types.KindInferenceModel, types.KindInferencePolicy, types.KindRetrievalModel},
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
	secret := summarizer.NewInferenceSecret("secret"+suffix, summarizerv1pb.InferenceSecretSpec_builder{
		Value: "my-secret-value",
	}.Build())

	model := summarizer.NewInferenceModel("model"+suffix, summarizerv1pb.InferenceModelSpec_builder{
		Openai: summarizerv1pb.OpenAIProvider_builder{
			OpenaiModelId:   "gpt-4o",
			ApiKeySecretRef: "secret" + suffix,
		}.Build(),
	}.Build())

	policy := summarizer.NewInferencePolicy("policy"+suffix, summarizerv1pb.InferencePolicySpec_builder{
		Kinds: []string{string(types.SSHSessionKind)},
		Model: "model" + suffix,
	}.Build())

	return secret, model, policy
}

func newBedrockModel(name string) *summarizerv1pb.InferenceModel {
	return summarizer.NewInferenceModel(name, summarizerv1pb.InferenceModelSpec_builder{
		Bedrock: summarizerv1pb.BedrockProvider_builder{
			Region:         "us-west-2",
			BedrockModelId: "anthropic.claude-3-5-sonnet-20240620-v1:0",
		}.Build(),
	}.Build())
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
	createdSecret, err := sclt.CreateInferenceSecret(ctx, summarizerv1pb.CreateInferenceSecretRequest_builder{
		Secret: secret,
	}.Build())
	require.NoError(t, err)
	assertResourceEquals(t, secret, createdSecret.GetSecret())

	// The secret value should have been written to the backend (we later test
	// retrieving through gRPC, but there we don't expect the secret value to be
	// returned).
	backendSecret, err := srv.AuthServer.AuthServer.GetInferenceSecret(ctx, "secret1")
	require.NoError(t, err)
	assertResourceEquals(t, secret, backendSecret)

	createdModel, err := sclt.CreateInferenceModel(ctx, summarizerv1pb.CreateInferenceModelRequest_builder{
		Model: expectedModel,
	}.Build())
	require.NoError(t, err)
	assertResourceEquals(t, expectedModel, createdModel.GetModel())

	createdPolicy, err := sclt.CreateInferencePolicy(ctx, summarizerv1pb.CreateInferencePolicyRequest_builder{
		Policy: expectedPolicy,
	}.Build())
	require.NoError(t, err)
	assertResourceEquals(t, expectedPolicy, createdPolicy.GetPolicy())

	// Test retrieving resources.
	gotSecret, err := sclt.GetInferenceSecret(ctx, summarizerv1pb.GetInferenceSecretRequest_builder{
		Name: "secret1",
	}.Build())
	require.NoError(t, err)

	// The returned secret should not contain the spec, as it is sensitive
	// information.
	expectedSecret := proto.CloneOf(secret)
	expectedSecret.ClearSpec()
	assertResourceEquals(t, expectedSecret, gotSecret.GetSecret())

	gotModel, err := sclt.GetInferenceModel(ctx, summarizerv1pb.GetInferenceModelRequest_builder{
		Name: "model1",
	}.Build())
	require.NoError(t, err)
	assertResourceEquals(t, expectedModel, gotModel.GetModel())

	// Test a valid Bedrock model. This one should be accepted.
	validBedrockModel := newBedrockModel("valid-bedrock-model")
	createdModel, err = sclt.CreateInferenceModel(ctx, summarizerv1pb.CreateInferenceModelRequest_builder{
		Model: validBedrockModel,
	}.Build())
	require.NoError(t, err)
	assertResourceEquals(t, validBedrockModel, createdModel.GetModel())

	gotModel, err = sclt.GetInferenceModel(ctx, summarizerv1pb.GetInferenceModelRequest_builder{
		Name: "valid-bedrock-model",
	}.Build())
	require.NoError(t, err)
	assertResourceEquals(t, validBedrockModel, gotModel.GetModel())

	// Test a Bedrock model that has a reserved name. This one should be
	// rejected.
	invalidBedrockModel := newBedrockModel(summarizer.CloudDefaultInferenceModelName)
	createdModel, err = sclt.CreateInferenceModel(ctx, summarizerv1pb.CreateInferenceModelRequest_builder{
		Model: invalidBedrockModel,
	}.Build())
	assert.ErrorIs(t, err, &trace.BadParameterError{
		Message: `metadata.name "teleport-cloud-default" is reserved`,
	})
	assert.Nil(t, createdModel)

	gotModel, err = sclt.GetInferenceModel(ctx, summarizerv1pb.GetInferenceModelRequest_builder{
		Name: summarizer.CloudDefaultInferenceModelName,
	}.Build())
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
	createdSecret, err := sclt.CreateInferenceSecret(ctx, summarizerv1pb.CreateInferenceSecretRequest_builder{
		Secret: secret,
	}.Build())
	require.NoError(t, err)

	createdModel, err := sclt.CreateInferenceModel(ctx, summarizerv1pb.CreateInferenceModelRequest_builder{
		Model: expectedModel,
	}.Build())
	require.NoError(t, err)

	createdPolicy, err := sclt.CreateInferencePolicy(ctx, summarizerv1pb.CreateInferencePolicyRequest_builder{
		Policy: expectedPolicy,
	}.Build())
	require.NoError(t, err)

	// Create the cloud default model directly in the backend, as it can't be
	// created via an API.
	_, err = srv.AuthServer.AuthServer.Summarizer.CreateInferenceModel(ctx, cloudDefaultModel)
	require.NoError(t, err)

	// Test updating the secret.
	createdSecret.GetSecret().GetSpec().SetValue("updated-secret-value")
	updatedSecret, err := sclt.UpdateInferenceSecret(ctx, summarizerv1pb.UpdateInferenceSecretRequest_builder{
		Secret: proto.CloneOf(createdSecret.GetSecret()),
	}.Build())
	require.NoError(t, err)

	// The secret should be updated in the backend.
	expectedSecret := proto.CloneOf(createdSecret.GetSecret())
	backendSecret, err := srv.AuthServer.AuthServer.GetInferenceSecret(ctx, "secret1")
	require.NoError(t, err)
	assertResourceEquals(t, expectedSecret, backendSecret)

	// The secret returned from the RPC should not contain the spec, as it is
	// sensitive information.
	expectedSecret.ClearSpec()
	assertResourceEquals(t, expectedSecret, updatedSecret.GetSecret())

	gotSecret, err := sclt.GetInferenceSecret(ctx, summarizerv1pb.GetInferenceSecretRequest_builder{
		Name: "secret1",
	}.Build())
	require.NoError(t, err)
	assertResourceEquals(t, expectedSecret, gotSecret.GetSecret())

	// Test updating the model.
	createdModel.GetModel().GetSpec().GetOpenai().SetTemperature(1.5)
	updatedModel, err := sclt.UpdateInferenceModel(ctx, summarizerv1pb.UpdateInferenceModelRequest_builder{
		Model: proto.CloneOf(createdModel.GetModel()),
	}.Build())
	require.NoError(t, err)
	assertResourceEquals(t, createdModel.GetModel(), updatedModel.GetModel())

	gotModel, err := sclt.GetInferenceModel(ctx, summarizerv1pb.GetInferenceModelRequest_builder{
		Name: "model1",
	}.Build())
	require.NoError(t, err)
	assertResourceEquals(t, createdModel.GetModel(), gotModel.GetModel())

	// Test attempting to update the cloud default model (should fail).
	modifiedCloudDefaultModel := proto.CloneOf(cloudDefaultModel)
	modifiedCloudDefaultModel.GetSpec().GetBedrock().SetTemperature(0.2)
	_, err = sclt.UpdateInferenceModel(ctx, summarizerv1pb.UpdateInferenceModelRequest_builder{
		Model: modifiedCloudDefaultModel,
	}.Build())
	assert.ErrorAs(t, err, new(*trace.BadParameterError))

	gotModel, err = sclt.GetInferenceModel(ctx, summarizerv1pb.GetInferenceModelRequest_builder{
		Name: "teleport-cloud-default",
	}.Build())
	require.NoError(t, err)
	assertResourceEquals(t, cloudDefaultModel, gotModel.GetModel())

	// Test updating the policy.
	createdPolicy.GetPolicy().GetSpec().SetFilter(`equals(resource.metadata.labels["env"], "dev")`)
	updatedPolicy, err := sclt.UpdateInferencePolicy(ctx, summarizerv1pb.UpdateInferencePolicyRequest_builder{
		Policy: proto.CloneOf(createdPolicy.GetPolicy()),
	}.Build())
	require.NoError(t, err)
	assertResourceEquals(t, createdPolicy.GetPolicy(), updatedPolicy.GetPolicy())

	gotPolicy, err := sclt.GetInferencePolicy(ctx, summarizerv1pb.GetInferencePolicyRequest_builder{
		Name: "policy1",
	}.Build())
	require.NoError(t, err)
	assertResourceEquals(t, createdPolicy.GetPolicy(), gotPolicy.GetPolicy())
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
	createdSecret, err := sclt.UpsertInferenceSecret(ctx, summarizerv1pb.UpsertInferenceSecretRequest_builder{
		Secret: secret,
	}.Build())
	require.NoError(t, err)

	// The secret returned from the RPC should not contain the spec, as it is
	// sensitive information.
	expectedSecret := proto.CloneOf(secret)
	expectedSecret.ClearSpec()
	assertResourceEquals(t, expectedSecret, createdSecret.GetSecret())

	createdModel, err := sclt.UpsertInferenceModel(ctx, summarizerv1pb.UpsertInferenceModelRequest_builder{
		Model: expectedModel,
	}.Build())
	require.NoError(t, err)

	createdPolicy, err := sclt.UpsertInferencePolicy(ctx, summarizerv1pb.UpsertInferencePolicyRequest_builder{
		Policy: expectedPolicy,
	}.Build())
	require.NoError(t, err)

	// Test updating the secret.
	secret.GetSpec().SetValue("updated-secret-value")
	updatedSecret, err := sclt.UpsertInferenceSecret(ctx, summarizerv1pb.UpsertInferenceSecretRequest_builder{
		Secret: proto.CloneOf(secret),
	}.Build())
	require.NoError(t, err)
	expectedSecret = proto.CloneOf(secret)
	expectedSecret.ClearSpec()
	assertResourceEquals(t, expectedSecret, updatedSecret.GetSecret())

	// The secret should be updated in the backend.
	backendSecret, err := srv.AuthServer.AuthServer.GetInferenceSecret(ctx, "secret1")
	require.NoError(t, err)
	assertResourceEquals(t, secret, backendSecret)

	gotSecret, err := sclt.GetInferenceSecret(ctx, summarizerv1pb.GetInferenceSecretRequest_builder{
		Name: "secret1",
	}.Build())
	require.NoError(t, err)
	assertResourceEquals(t, expectedSecret, gotSecret.GetSecret())

	// Test updating the model.
	createdModel.GetModel().GetSpec().GetOpenai().SetTemperature(1.5)
	updatedModel, err := sclt.UpsertInferenceModel(ctx, summarizerv1pb.UpsertInferenceModelRequest_builder{
		Model: proto.CloneOf(createdModel.GetModel()),
	}.Build())
	require.NoError(t, err)
	assertResourceEquals(t, createdModel.GetModel(), updatedModel.GetModel())

	gotModel, err := sclt.GetInferenceModel(ctx, summarizerv1pb.GetInferenceModelRequest_builder{
		Name: "model1",
	}.Build())
	require.NoError(t, err)
	assertResourceEquals(t, createdModel.GetModel(), gotModel.GetModel())

	// Test attempting to update the cloud default model (should fail).
	modifiedCloudDefaultModel := proto.CloneOf(cloudDefaultModel)
	modifiedCloudDefaultModel.GetSpec().GetBedrock().SetTemperature(0.2)
	_, err = sclt.UpsertInferenceModel(ctx, summarizerv1pb.UpsertInferenceModelRequest_builder{
		Model: modifiedCloudDefaultModel,
	}.Build())
	assert.ErrorAs(t, err, new(*trace.BadParameterError))

	gotModel, err = sclt.GetInferenceModel(ctx, summarizerv1pb.GetInferenceModelRequest_builder{
		Name: "teleport-cloud-default",
	}.Build())
	require.NoError(t, err)
	assertResourceEquals(t, cloudDefaultModel, gotModel.GetModel())

	// Test updating the policy.
	createdPolicy.GetPolicy().GetSpec().SetFilter(`equals(resource.metadata.labels["env"], "dev")`)
	updatedPolicy, err := sclt.UpsertInferencePolicy(ctx, summarizerv1pb.UpsertInferencePolicyRequest_builder{
		Policy: proto.CloneOf(createdPolicy.GetPolicy()),
	}.Build())
	require.NoError(t, err)
	assertResourceEquals(t, createdPolicy.GetPolicy(), updatedPolicy.GetPolicy())

	gotPolicy, err := sclt.GetInferencePolicy(ctx, summarizerv1pb.GetInferencePolicyRequest_builder{
		Name: "policy1",
	}.Build())
	require.NoError(t, err)
	assertResourceEquals(t, createdPolicy.GetPolicy(), gotPolicy.GetPolicy())
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
	_, err = sclt.CreateInferenceSecret(ctx, summarizerv1pb.CreateInferenceSecretRequest_builder{
		Secret: secret,
	}.Build())
	require.NoError(t, err)

	_, err = sclt.CreateInferenceModel(ctx, summarizerv1pb.CreateInferenceModelRequest_builder{
		Model: expectedModel,
	}.Build())
	require.NoError(t, err)

	_, err = sclt.CreateInferencePolicy(ctx, summarizerv1pb.CreateInferencePolicyRequest_builder{
		Policy: expectedPolicy,
	}.Build())
	require.NoError(t, err)

	// Create the cloud default model directly in the backend, as it can't be
	// created via an API.
	_, err = srv.AuthServer.AuthServer.Summarizer.CreateInferenceModel(ctx, cloudDefaultModel)
	require.NoError(t, err)

	// Delete the resources.
	_, err = sclt.DeleteInferencePolicy(ctx, summarizerv1pb.DeleteInferencePolicyRequest_builder{
		Name: "policy1",
	}.Build())
	require.NoError(t, err)

	_, err = sclt.DeleteInferenceModel(ctx, summarizerv1pb.DeleteInferenceModelRequest_builder{
		Name: "model1",
	}.Build())
	require.NoError(t, err)

	_, err = sclt.DeleteInferenceSecret(ctx, summarizerv1pb.DeleteInferenceSecretRequest_builder{
		Name: "secret1",
	}.Build())
	require.NoError(t, err)

	// Verify that the resources are deleted.
	_, err = sclt.GetInferencePolicy(ctx, summarizerv1pb.GetInferencePolicyRequest_builder{
		Name: "policy1",
	}.Build())
	require.Error(t, err)

	assert.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)
	_, err = sclt.GetInferenceModel(ctx, summarizerv1pb.GetInferenceModelRequest_builder{
		Name: "model1",
	}.Build())
	require.Error(t, err)

	assert.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)
	_, err = sclt.GetInferenceSecret(ctx, summarizerv1pb.GetInferenceSecretRequest_builder{
		Name: "secret1",
	}.Build())
	require.Error(t, err)
	assert.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)

	// Test attempting to delete the cloud default model (should fail).
	_, err = sclt.DeleteInferenceModel(ctx, summarizerv1pb.DeleteInferenceModelRequest_builder{
		Name: "teleport-cloud-default",
	}.Build())
	assert.ErrorAs(t, err, new(*trace.BadParameterError))

	gotModel, err := sclt.GetInferenceModel(ctx, summarizerv1pb.GetInferenceModelRequest_builder{
		Name: "teleport-cloud-default",
	}.Build())
	require.NoError(t, err)
	assertResourceEquals(t, cloudDefaultModel, gotModel.GetModel())
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

		_, err = sclt.CreateInferenceSecret(ctx, summarizerv1pb.CreateInferenceSecretRequest_builder{
			Secret: secret,
		}.Build())
		require.NoError(t, err)
		secret.ClearSpec() // The secret spec is sensitive and should not be returned in the list.
		expectedSecrets = append(expectedSecrets, secret)

		_, err = sclt.CreateInferenceModel(ctx, summarizerv1pb.CreateInferenceModelRequest_builder{
			Model: expectedModel,
		}.Build())
		require.NoError(t, err)
		expectedModels = append(expectedModels, expectedModel)

		_, err = sclt.CreateInferencePolicy(ctx, summarizerv1pb.CreateInferencePolicyRequest_builder{
			Policy: expectedPolicy,
		}.Build())
		require.NoError(t, err)
		expectedPolicies = append(expectedPolicies, expectedPolicy)
	}

	// List secrets.
	allSecrets := []*summarizerv1pb.InferenceSecret{}
	secretsPage, err := sclt.ListInferenceSecrets(ctx, summarizerv1pb.ListInferenceSecretsRequest_builder{
		PageSize: 2,
	}.Build())
	require.NoError(t, err)
	assert.Len(t, secretsPage.GetSecrets(), 2)
	allSecrets = append(allSecrets, secretsPage.GetSecrets()...)

	secretsPage, err = sclt.ListInferenceSecrets(ctx, summarizerv1pb.ListInferenceSecretsRequest_builder{
		PageSize:  2,
		PageToken: secretsPage.GetNextPageToken(),
	}.Build())
	require.NoError(t, err)
	assert.Len(t, secretsPage.GetSecrets(), 1)
	allSecrets = append(allSecrets, secretsPage.GetSecrets()...)

	assertResourceListEqualsIgnoringOrder(t, expectedSecrets, allSecrets)

	// List models.
	allModels := []*summarizerv1pb.InferenceModel{}
	modelsPage, err := sclt.ListInferenceModels(ctx, summarizerv1pb.ListInferenceModelsRequest_builder{
		PageSize: 2,
	}.Build())
	require.NoError(t, err)
	assert.Len(t, modelsPage.GetModels(), 2)
	allModels = append(allModels, modelsPage.GetModels()...)

	modelsPage, err = sclt.ListInferenceModels(ctx, summarizerv1pb.ListInferenceModelsRequest_builder{
		PageSize:  2,
		PageToken: modelsPage.GetNextPageToken(),
	}.Build())
	require.NoError(t, err)
	assert.Len(t, modelsPage.GetModels(), 1)
	allModels = append(allModels, modelsPage.GetModels()...)

	assertResourceListEqualsIgnoringOrder(t, expectedModels, allModels)

	// List policies.
	allPolicies := []*summarizerv1pb.InferencePolicy{}
	policiesPage, err := sclt.ListInferencePolicies(ctx, summarizerv1pb.ListInferencePoliciesRequest_builder{
		PageSize: 2,
	}.Build())
	require.NoError(t, err)
	assert.Len(t, policiesPage.GetPolicies(), 2)
	allPolicies = append(allPolicies, policiesPage.GetPolicies()...)

	policiesPage, err = sclt.ListInferencePolicies(ctx, summarizerv1pb.ListInferencePoliciesRequest_builder{
		PageSize:  2,
		PageToken: policiesPage.GetNextPageToken(),
	}.Build())
	require.NoError(t, err)
	assert.Len(t, policiesPage.GetPolicies(), 1)
	allPolicies = append(allPolicies, policiesPage.GetPolicies()...)

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
	createdSecret, err := sclt.CreateInferenceSecret(ctx, summarizerv1pb.CreateInferenceSecretRequest_builder{
		Secret: secret,
	}.Build())
	require.NoError(t, err)
	createdModel, err := sclt.CreateInferenceModel(ctx, summarizerv1pb.CreateInferenceModelRequest_builder{
		Model: model,
	}.Build())
	require.NoError(t, err)
	createdPolicy, err := sclt.CreateInferencePolicy(ctx, summarizerv1pb.CreateInferencePolicyRequest_builder{
		Policy: policy,
	}.Build())
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
				_, err := sclt.CreateInferenceSecret(ctx, summarizerv1pb.CreateInferenceSecretRequest_builder{
					Secret: secret2,
				}.Build())
				return err
			},
		},
		{
			name:     "create model",
			resource: types.KindInferenceModel,
			verbs:    []string{types.VerbCreate},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.CreateInferenceModel(ctx, summarizerv1pb.CreateInferenceModelRequest_builder{
					Model: model2,
				}.Build())
				return err
			},
		},
		{
			name:     "create policy",
			resource: types.KindInferencePolicy,
			verbs:    []string{types.VerbCreate},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.CreateInferencePolicy(ctx, summarizerv1pb.CreateInferencePolicyRequest_builder{
					Policy: policy2,
				}.Build())
				return err
			},
		},
		{
			name:     "get secret",
			resource: types.KindInferenceSecret,
			verbs:    []string{types.VerbRead},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.GetInferenceSecret(ctx, summarizerv1pb.GetInferenceSecretRequest_builder{
					Name: "secret1",
				}.Build())
				return err
			},
		},
		{
			name:     "get model",
			resource: types.KindInferenceModel,
			verbs:    []string{types.VerbRead},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.GetInferenceModel(ctx, summarizerv1pb.GetInferenceModelRequest_builder{
					Name: "model1",
				}.Build())
				return err
			},
		},
		{
			name:     "get policy",
			resource: types.KindInferencePolicy,
			verbs:    []string{types.VerbRead},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.GetInferencePolicy(ctx, summarizerv1pb.GetInferencePolicyRequest_builder{
					Name: "policy1",
				}.Build())
				return err
			},
		},
		{
			name:     "update secret",
			resource: types.KindInferenceSecret,
			verbs:    []string{types.VerbUpdate},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.UpdateInferenceSecret(ctx, summarizerv1pb.UpdateInferenceSecretRequest_builder{
					Secret: createdSecret.GetSecret(),
				}.Build())
				return err
			},
		},
		{
			name:     "update model",
			resource: types.KindInferenceModel,
			verbs:    []string{types.VerbUpdate},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.UpdateInferenceModel(ctx, summarizerv1pb.UpdateInferenceModelRequest_builder{
					Model: createdModel.GetModel(),
				}.Build())
				return err
			},
		},
		{
			name:     "update policy",
			resource: types.KindInferencePolicy,
			verbs:    []string{types.VerbUpdate},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.UpdateInferencePolicy(ctx, summarizerv1pb.UpdateInferencePolicyRequest_builder{
					Policy: createdPolicy.GetPolicy(),
				}.Build())
				return err
			},
		},
		{
			name:     "upsert secret",
			resource: types.KindInferenceSecret,
			verbs:    []string{types.VerbCreate, types.VerbUpdate},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.UpsertInferenceSecret(ctx, summarizerv1pb.UpsertInferenceSecretRequest_builder{
					Secret: createdSecret.GetSecret(),
				}.Build())
				return err
			},
		},
		{
			name:     "upsert model",
			resource: types.KindInferenceModel,
			verbs:    []string{types.VerbCreate, types.VerbUpdate},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.UpsertInferenceModel(ctx, summarizerv1pb.UpsertInferenceModelRequest_builder{
					Model: createdModel.GetModel(),
				}.Build())
				return err
			},
		},
		{
			name:     "upsert policy",
			resource: types.KindInferencePolicy,
			verbs:    []string{types.VerbCreate, types.VerbUpdate},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.UpsertInferencePolicy(ctx, summarizerv1pb.UpsertInferencePolicyRequest_builder{
					Policy: createdPolicy.GetPolicy(),
				}.Build())
				return err
			},
		},
		{
			name:     "delete secret",
			resource: types.KindInferenceSecret,
			verbs:    []string{types.VerbDelete},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.DeleteInferenceSecret(ctx, summarizerv1pb.DeleteInferenceSecretRequest_builder{
					Name: "secret1",
				}.Build())
				return err
			},
		},
		{
			name:     "delete model",
			resource: types.KindInferenceModel,
			verbs:    []string{types.VerbDelete},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.DeleteInferenceModel(ctx, summarizerv1pb.DeleteInferenceModelRequest_builder{
					Name: "model1",
				}.Build())
				return err
			},
		},
		{
			name:     "delete policy",
			resource: types.KindInferencePolicy,
			verbs:    []string{types.VerbDelete},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.DeleteInferencePolicy(ctx, summarizerv1pb.DeleteInferencePolicyRequest_builder{
					Name: "policy1",
				}.Build())
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

	return summarizerv1pb.Summary_builder{
		SessionId:           sessionEnd.SessionID,
		State:               summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS,
		InferenceStartedAt:  timestamppb.New(inferenceStartTime),
		InferenceFinishedAt: timestamppb.New(inferenceEndTime),
		Content:             "This is a test summary content.",
		ModelName:           "some-model",
		SessionEndEvent:     endEventStruct,
	}.Build()
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
		summary1.SetContent("")
		summary1.ClearInferenceFinishedAt()
		//nolint:staticcheck // SA1019. Pending state is deprecated but will be replaced with other states.
		summary1.SetState(summarizerv1pb.SummaryState_SUMMARY_STATE_PENDING)
		b, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(summary1)
		require.NoError(t, err)
		srv.AuthServer.AuthServer.UploadPendingSummary(ctx, session.ID(summary1.GetSessionId()), bytes.NewReader(b))

		// Summary 2 is in a final state.
		session2End := newSessionEndEvent()
		summary2 := newTestSummary(t, session2End)
		summary2Pending := proto.CloneOf(summary2)
		summary2Pending.SetContent("")
		summary2Pending.ClearInferenceFinishedAt()
		//nolint:staticcheck // SA1019. Pending state is deprecated but will be replaced with other states.
		summary2Pending.SetState(summarizerv1pb.SummaryState_SUMMARY_STATE_PENDING)

		// Upload the pending state of summary 2.
		b, err = protojson.MarshalOptions{UseProtoNames: true}.Marshal(summary2Pending)
		require.NoError(t, err)
		_, err = srv.AuthServer.AuthServer.UploadPendingSummary(ctx, session.ID(summary2Pending.GetSessionId()), bytes.NewReader(b))
		require.NoError(t, err)

		// Upload the final state of summary 2.
		b, err = protojson.MarshalOptions{UseProtoNames: true}.Marshal(summary2)
		require.NoError(t, err)
		_, err = srv.AuthServer.AuthServer.UploadSummary(ctx, session.ID(summary2.GetSessionId()), bytes.NewReader(b))
		require.NoError(t, err)

		// Test fetching a pending summary.
		got, err := sclt.GetSummary(ctx, summarizerv1pb.GetSummaryRequest_builder{
			SessionId: summary1.GetSessionId(),
		}.Build())
		require.NoError(t, err)
		assert.Empty(t, cmp.Diff(summary1, got.GetSummary(), protocmp.Transform()))

		// Test fetching a final summary.
		got, err = sclt.GetSummary(ctx, summarizerv1pb.GetSummaryRequest_builder{
			SessionId: summary2.GetSessionId(),
		}.Build())
		require.NoError(t, err)
		assert.Empty(t, cmp.Diff(summary2, got.GetSummary(), protocmp.Transform()))

		// Make sure that the session end event can be fully recovered from the
		// unstructured representation.
		gotSessionEnd, err := events.FromEventFields(got.GetSummary().GetSessionEndEvent().AsMap())
		require.NoError(t, err)
		assert.Empty(t, cmp.Diff(session2End, gotSessionEnd, protocmp.Transform()))

		// Test fetching a summary that doesn't exist.
		_, err = sclt.GetSummary(ctx, summarizerv1pb.GetSummaryRequest_builder{
			SessionId: uuid.NewString(),
		}.Build())
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

		srv.AuthServer.AuthServer.UploadSummary(ctx, session.ID(expectedSummary.GetSessionId()), bytes.NewReader(encrypt(b)))
		// Test fetching an existing encrypted summary.
		got, err := sclt.GetSummary(ctx, summarizerv1pb.GetSummaryRequest_builder{
			SessionId: expectedSummary.GetSessionId(),
		}.Build())
		require.NoError(t, err)
		assert.Empty(t, cmp.Diff(expectedSummary, got.GetSummary(), protocmp.Transform()))
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
	srv.AuthServer.AuthServer.UploadSummary(ctx, session.ID(summary1.GetSessionId()), bytes.NewReader(b))

	// Session of Bob and Mary
	sessionEnd2 := newSessionEndEvent()
	sessionEnd2.SessionID = "0fd10888-a48d-45b7-9a10-dc4321f7b159"
	sessionEnd2.UserMetadata.User = "bob"
	sessionEnd2.Participants = []string{"bob", "mary"}
	summary2 := newTestSummary(t, sessionEnd2)
	b, err = protojson.MarshalOptions{UseProtoNames: true}.Marshal(summary2)
	require.NoError(t, err)
	srv.AuthServer.AuthServer.UploadSummary(ctx, session.ID(summary2.GetSessionId()), bytes.NewReader(b))

	// Add Alice.
	createTestUser(t, srv, "alice")
	aliceClt, err := srv.NewClient(authtest.TestUser("alice"))
	require.NoError(t, err)
	aliceSclt := aliceClt.SummarizerServiceClient()

	// Alice should only see the first session (see the "where" condition in
	// user's role).
	_, err = aliceSclt.GetSummary(ctx, summarizerv1pb.GetSummaryRequest_builder{
		SessionId: summary1.GetSessionId(),
	}.Build())
	require.NoError(t, err)
	_, err = aliceSclt.GetSummary(ctx, summarizerv1pb.GetSummaryRequest_builder{
		SessionId: summary2.GetSessionId(),
	}.Build())
	require.Error(t, err)
	assert.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, got %v", err)

	// Add Bob.
	createTestUser(t, srv, "bob")
	bobClt, err := srv.NewClient(authtest.TestUser("bob"))
	require.NoError(t, err)
	bobSclt := bobClt.SummarizerServiceClient()

	// Bob should be able to see both sessions, as he participated in both.
	_, err = bobSclt.GetSummary(ctx, summarizerv1pb.GetSummaryRequest_builder{
		SessionId: summary1.GetSessionId(),
	}.Build())
	require.NoError(t, err)
	_, err = bobSclt.GetSummary(ctx, summarizerv1pb.GetSummaryRequest_builder{
		SessionId: summary2.GetSessionId(),
	}.Build())
	require.NoError(t, err)

	// Add Mary.
	createTestUser(t, srv, "mary")
	maryClt, err := srv.NewClient(authtest.TestUser("mary"))
	require.NoError(t, err)
	marySclt := maryClt.SummarizerServiceClient()

	// Mary should only see the second session (see the "where" condition in
	// user's role).
	_, err = marySclt.GetSummary(ctx, summarizerv1pb.GetSummaryRequest_builder{
		SessionId: summary1.GetSessionId(),
	}.Build())
	require.Error(t, err)
	assert.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, got %v", err)
	_, err = marySclt.GetSummary(ctx, summarizerv1pb.GetSummaryRequest_builder{
		SessionId: summary2.GetSessionId(),
	}.Build())
	require.NoError(t, err)

	// Add an account that doesn't have any access to session recordings (there's
	// a special case for that in the code).
	_, _, err = authtest.CreateUserAndRole(srv.Auth(), "intern", []string{}, []types.Rule{})
	require.NoError(t, err)
	internClt, err := srv.NewClient(authtest.TestUser("intern"))
	require.NoError(t, err)
	internSclt := internClt.SummarizerServiceClient()

	_, err = internSclt.GetSummary(ctx, summarizerv1pb.GetSummaryRequest_builder{
		SessionId: summary1.GetSessionId(),
	}.Build())
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

	_, err = canViewUser.SummarizerServiceClient().GetSummary(ctx, summarizerv1pb.GetSummaryRequest_builder{
		SessionId: summary1.GetSessionId(),
	}.Build())
	require.NoError(t, err)

	// Remove the node_labels condition from the role, which should make the
	// user lose access to everything (because of the "can_view()" condition).
	canViewUserRole.SetNodeLabels(types.Allow, nil)
	_, err = srv.Auth().UpdateRole(ctx, canViewUserRole)
	require.NoError(t, err)

	_, err = canViewUser.SummarizerServiceClient().GetSummary(ctx, summarizerv1pb.GetSummaryRequest_builder{
		SessionId: summary1.GetSessionId(),
	}.Build())
	require.Error(t, err)
	assert.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, got %v", err)
}

func TestService_BatchGetSummaryMetadata(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	usageReporter := &fakeUsageReporter{}
	srv := newTestTLSServer(t, withUsageReporter(usageReporter))
	createTestUser(t, srv, "alice")

	clt, err := srv.NewClient(authtest.TestUser("alice"))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()

	// Session of Alice and Bob, with enhanced summary data.
	sessionEnd1 := newSessionEndEvent()
	summary1 := newTestSummary(t, sessionEnd1)
	summary1.SetEnhancedSummary(summarizerv1pb.EnhancedSummary_builder{
		RiskLevel: summarizerv1pb.RiskLevel_RISK_LEVEL_HIGH,
		NeedsFurtherReviewReasons: []summarizerv1pb.NeedsReviewReason{
			summarizerv1pb.NeedsReviewReason_NEEDS_REVIEW_REASON_TOO_LARGE,
		},
	}.Build())
	b, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(summary1)
	require.NoError(t, err)
	_, err = srv.AuthServer.AuthServer.UploadSummary(ctx, session.ID(summary1.GetSessionId()), bytes.NewReader(b))
	require.NoError(t, err)

	// Session of Bob and Mary; Alice has no access to it.
	sessionEnd2 := newSessionEndEvent()
	sessionEnd2.UserMetadata.User = "bob"
	sessionEnd2.Participants = []string{"bob", "mary"}
	summary2 := newTestSummary(t, sessionEnd2)
	b, err = protojson.MarshalOptions{UseProtoNames: true}.Marshal(summary2)
	require.NoError(t, err)
	_, err = srv.AuthServer.AuthServer.UploadSummary(ctx, session.ID(summary2.GetSessionId()), bytes.NewReader(b))
	require.NoError(t, err)

	// Session of Alice and Bob, carrying only the deprecated singular review reason, as written by a pre-v19 auth.
	sessionEnd3 := newSessionEndEvent()
	summary3 := newTestSummary(t, sessionEnd3)
	es3 := summarizerv1pb.EnhancedSummary_builder{
		RiskLevel: summarizerv1pb.RiskLevel_RISK_LEVEL_LOW,
	}.Build()
	//nolint:staticcheck // deprecated field write to simulate a pre-v19 summary
	es3.SetNeedsFurtherReview(summarizerv1pb.NeedsReviewReason_NEEDS_REVIEW_REASON_COMMAND_ANALYSIS_FAILED)
	summary3.SetEnhancedSummary(es3)
	b, err = protojson.MarshalOptions{UseProtoNames: true}.Marshal(summary3)
	require.NoError(t, err)
	_, err = srv.AuthServer.AuthServer.UploadSummary(ctx, session.ID(summary3.GetSessionId()), bytes.NewReader(b))
	require.NoError(t, err)

	t.Run("mixed batch", func(t *testing.T) {
		got, err := sclt.BatchGetSummaryMetadata(ctx, summarizerv1pb.BatchGetSummaryMetadataRequest_builder{
			SessionIds: []string{
				summary1.GetSessionId(),
				summary2.GetSessionId(), // Inaccessible; omitted.
				summary3.GetSessionId(),
				uuid.NewString(), // Nonexistent; omitted.
			},
		}.Build())
		require.NoError(t, err)

		want := []*summarizerv1pb.SummaryMetadata{
			summarizerv1pb.SummaryMetadata_builder{
				SessionId: summary1.GetSessionId(),
				State:     summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS,
				RiskLevel: summarizerv1pb.RiskLevel_RISK_LEVEL_HIGH,
				NeedsFurtherReviewReasons: []summarizerv1pb.NeedsReviewReason{
					summarizerv1pb.NeedsReviewReason_NEEDS_REVIEW_REASON_TOO_LARGE,
				},
			}.Build(),
			summarizerv1pb.SummaryMetadata_builder{
				SessionId: summary3.GetSessionId(),
				State:     summarizerv1pb.SummaryState_SUMMARY_STATE_SUCCESS,
				RiskLevel: summarizerv1pb.RiskLevel_RISK_LEVEL_LOW,
				NeedsFurtherReviewReasons: []summarizerv1pb.NeedsReviewReason{
					summarizerv1pb.NeedsReviewReason_NEEDS_REVIEW_REASON_COMMAND_ANALYSIS_FAILED,
				},
			}.Build(),
		}
		assert.Empty(t, cmp.Diff(want, got.GetMetadata(), protocmp.Transform()))
	})

	t.Run("empty request", func(t *testing.T) {
		got, err := sclt.BatchGetSummaryMetadata(ctx, summarizerv1pb.BatchGetSummaryMetadataRequest_builder{}.Build())
		require.NoError(t, err)
		assert.Empty(t, got.GetMetadata())
	})

	t.Run("too many sessions", func(t *testing.T) {
		ids := make([]string, maxBatchGetSummaryMetadataSessions+1)
		for i := range ids {
			ids[i] = uuid.NewString()
		}
		_, err := sclt.BatchGetSummaryMetadata(ctx, summarizerv1pb.BatchGetSummaryMetadataRequest_builder{
			SessionIds: ids,
		}.Build())
		require.Error(t, err)
		assert.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)
	})

	t.Run("no access to any sessions", func(t *testing.T) {
		_, _, err := authtest.CreateUserAndRole(srv.Auth(), "intern", []string{}, []types.Rule{})
		require.NoError(t, err)
		internClt, err := srv.NewClient(authtest.TestUser("intern"))
		require.NoError(t, err)

		_, err = internClt.SummarizerServiceClient().BatchGetSummaryMetadata(ctx,
			summarizerv1pb.BatchGetSummaryMetadataRequest_builder{
				SessionIds: []string{summary1.GetSessionId()},
			}.Build())
		require.Error(t, err)
		assert.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, got %v", err)
	})

	// Metadata reads decorate session lists and should not count as summary accesses.
	for _, event := range usageReporter.events {
		_, ok := event.(*usagereporter.SessionSummaryAccessEvent)
		assert.False(t, ok, "BatchGetSummaryMetadata should not emit summary access usage events")
	}
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

		_, err := sclt.CreateInferenceSecret(ctx, summarizerv1pb.CreateInferenceSecretRequest_builder{
			Secret: secret,
		}.Build())
		require.NoError(t, err)

		createdModel, err := sclt.CreateInferenceModel(ctx, summarizerv1pb.CreateInferenceModelRequest_builder{
			Model: model,
		}.Build())
		require.NoError(t, err)

		evts := getRecentEvents(events.InferenceModelCreateEvent)
		require.NotEmpty(t, evts, "expected at least one model create event")
		createEvt, ok := evts[len(evts)-1].(*apievents.InferenceModelCreate)
		require.True(t, ok, "expected InferenceModelCreate event, got %T", evts[len(evts)-1])
		assert.Equal(t, events.InferenceModelCreateCode, createEvt.Code)
		assert.Equal(t, "modelaudit1", createEvt.ResourceMetadata.Name)
		assert.Equal(t, user.GetName(), createEvt.User)

		createdModel.GetModel().GetSpec().GetOpenai().SetTemperature(1.5)
		_, err = sclt.UpdateInferenceModel(ctx, summarizerv1pb.UpdateInferenceModelRequest_builder{
			Model: createdModel.GetModel(),
		}.Build())
		require.NoError(t, err)

		evts = getRecentEvents(events.InferenceModelUpdateEvent)
		require.NotEmpty(t, evts, "expected at least one model update event")
		updateEvt, ok := evts[len(evts)-1].(*apievents.InferenceModelUpdate)
		require.True(t, ok, "expected InferenceModelUpdate event got %T", evts[len(evts)-1])
		assert.Equal(t, events.InferenceModelUpdateCode, updateEvt.Code)
		assert.Equal(t, "modelaudit1", updateEvt.ResourceMetadata.Name)
		assert.Equal(t, user.GetName(), updateEvt.User)

		_, err = sclt.DeleteInferenceModel(ctx, summarizerv1pb.DeleteInferenceModelRequest_builder{
			Name: "modelaudit1",
		}.Build())
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

		_, err := sclt.CreateInferenceSecret(ctx, summarizerv1pb.CreateInferenceSecretRequest_builder{
			Secret: secret,
		}.Build())
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
		backendSecret.GetSpec().SetValue("updated-value")
		_, err = sclt.UpdateInferenceSecret(ctx, summarizerv1pb.UpdateInferenceSecretRequest_builder{
			Secret: backendSecret,
		}.Build())
		require.NoError(t, err)

		evts = getRecentEvents(events.InferenceSecretUpdateEvent)
		require.NotEmpty(t, evts, "expected at least one secret update event")
		updateEvt, ok := evts[len(evts)-1].(*apievents.InferenceSecretUpdate)
		require.True(t, ok, "expected InferenceSecretUpdate event, got %T", evts[len(evts)-1])
		assert.Equal(t, events.InferenceSecretUpdateCode, updateEvt.Code)
		assert.Equal(t, "secretaudit2", updateEvt.ResourceMetadata.Name)
		assert.Equal(t, user.GetName(), updateEvt.User)

		_, err = sclt.DeleteInferenceSecret(ctx, summarizerv1pb.DeleteInferenceSecretRequest_builder{
			Name: "secretaudit2",
		}.Build())
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

		_, err := sclt.CreateInferenceSecret(ctx, summarizerv1pb.CreateInferenceSecretRequest_builder{
			Secret: secret,
		}.Build())
		require.NoError(t, err)
		_, err = sclt.CreateInferenceModel(ctx, summarizerv1pb.CreateInferenceModelRequest_builder{
			Model: model,
		}.Build())
		require.NoError(t, err)

		createdPolicy, err := sclt.CreateInferencePolicy(ctx, summarizerv1pb.CreateInferencePolicyRequest_builder{
			Policy: policy,
		}.Build())
		require.NoError(t, err)

		evts := getRecentEvents(events.InferencePolicyCreateEvent)
		require.NotEmpty(t, evts, "expected at least one policy create event")
		createEvt, ok := evts[len(evts)-1].(*apievents.InferencePolicyCreate)
		require.True(t, ok, "expected InferencePolicyCreate event, got %T", evts[len(evts)-1])
		assert.Equal(t, events.InferencePolicyCreateCode, createEvt.Code)
		assert.Equal(t, "policyaudit3", createEvt.ResourceMetadata.Name)
		assert.Equal(t, user.GetName(), createEvt.User)

		createdPolicy.GetPolicy().GetSpec().SetFilter(`equals(resource.metadata.labels["env"], "dev")`)
		_, err = sclt.UpdateInferencePolicy(ctx, summarizerv1pb.UpdateInferencePolicyRequest_builder{
			Policy: createdPolicy.GetPolicy(),
		}.Build())
		require.NoError(t, err)

		evts = getRecentEvents(events.InferencePolicyUpdateEvent)
		require.NotEmpty(t, evts, "expected at least one policy update event")
		updateEvt, ok := evts[len(evts)-1].(*apievents.InferencePolicyUpdate)
		require.True(t, ok, "expected InferencePolicyUpdate event, got %T", evts[len(evts)-1])
		assert.Equal(t, events.InferencePolicyUpdateCode, updateEvt.Code)
		assert.Equal(t, "policyaudit3", updateEvt.ResourceMetadata.Name)
		assert.Equal(t, user.GetName(), updateEvt.User)

		_, err = sclt.DeleteInferencePolicy(ctx, summarizerv1pb.DeleteInferencePolicyRequest_builder{
			Name: "policyaudit3",
		}.Build())
		require.NoError(t, err)

		evts = getRecentEvents(events.InferencePolicyDeleteEvent)
		require.NotEmpty(t, evts, "expected at least one policy delete event")
		deleteEvt, ok := evts[len(evts)-1].(*apievents.InferencePolicyDelete)
		require.True(t, ok, "expected InferencePolicyDelete event, got %T", evts[len(evts)-1])
		assert.Equal(t, events.InferencePolicyDeleteCode, deleteEvt.Code)
		assert.Equal(t, "policyaudit3", deleteEvt.ResourceMetadata.Name)
		assert.Equal(t, user.GetName(), deleteEvt.User)
	})

	t.Run("RetrievalModel events", func(t *testing.T) {
		secret := summarizer.NewInferenceSecret("secret1", summarizerv1pb.InferenceSecretSpec_builder{
			Value: "my-secret-value",
		}.Build())
		model := newBedrockModel("test-inference-model")
		retrievalModel := newTestRetrievalModel()

		_, err := sclt.CreateInferenceSecret(ctx, summarizerv1pb.CreateInferenceSecretRequest_builder{
			Secret: secret,
		}.Build())
		require.NoError(t, err)
		_, err = sclt.CreateInferenceModel(ctx, summarizerv1pb.CreateInferenceModelRequest_builder{
			Model: model,
		}.Build())
		require.NoError(t, err)

		createdRetrievalModel, err := sclt.CreateRetrievalModel(ctx, summarizerv1pb.CreateRetrievalModelRequest_builder{
			Model: retrievalModel,
		}.Build())
		require.NoError(t, err)

		evts := getRecentEvents(events.RetrievalModelCreateEvent)
		require.NotEmpty(t, evts, "expected at least one retrieval model create event")
		createEvt, ok := evts[len(evts)-1].(*apievents.RetrievalModelCreate)
		require.True(t, ok, "expected RetrievalModelCreate event, got %T", evts[len(evts)-1])
		assert.Equal(t, events.RetrievalModelCreateCode, createEvt.Code)
		assert.Equal(t, types.MetaNameRetrievalModel, createEvt.ResourceMetadata.Name)
		assert.Equal(t, user.GetName(), createEvt.User)

		createdRetrievalModel.GetModel().GetSpec().GetOpenai().SetTemperature(0.5)
		_, err = sclt.UpdateRetrievalModel(ctx, summarizerv1pb.UpdateRetrievalModelRequest_builder{
			Model: createdRetrievalModel.GetModel(),
		}.Build())
		require.NoError(t, err)

		evts = getRecentEvents(events.RetrievalModelUpdateEvent)
		require.NotEmpty(t, evts, "expected at least one retrieval model update event")
		updateEvt, ok := evts[len(evts)-1].(*apievents.RetrievalModelUpdate)
		require.True(t, ok, "expected RetrievalModelUpdate event got %T", evts[len(evts)-1])
		assert.Equal(t, events.RetrievalModelUpdateCode, updateEvt.Code)
		assert.Equal(t, types.MetaNameRetrievalModel, updateEvt.ResourceMetadata.Name)
		assert.Equal(t, user.GetName(), updateEvt.User)

		_, err = sclt.DeleteRetrievalModel(ctx, &summarizerv1pb.DeleteRetrievalModelRequest{})
		require.NoError(t, err)

		evts = getRecentEvents(events.RetrievalModelDeleteEvent)
		require.NotEmpty(t, evts, "expected at least one retrieval model delete event")
		deleteEvt, ok := evts[len(evts)-1].(*apievents.RetrievalModelDelete)
		require.True(t, ok, "expected RetrievalModelDelete event got %T", evts[len(evts)-1])
		assert.Equal(t, events.RetrievalModelDeleteCode, deleteEvt.Code)
		assert.Equal(t, types.MetaNameRetrievalModel, deleteEvt.ResourceMetadata.Name)
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
			req: summarizerv1pb.TestInferenceModelRequest_builder{
				Model: nil,
			}.Build(),
			expectSuccess:   false,
			messageContains: "model spec is required",
		},
		{
			name: "OpenAI without secret",
			req: summarizerv1pb.TestInferenceModelRequest_builder{
				Model: summarizerv1pb.InferenceModelSpec_builder{
					Openai: summarizerv1pb.OpenAIProvider_builder{
						OpenaiModelId: "gpt-4o",
					}.Build(),
				}.Build(),
			}.Build(),
			expectSuccess:   false,
			messageContains: "api_key_secret_ref is required for OpenAI models when no secret is provided in the request",
		},
		{
			name: "OpenAI with invalid API key",
			req: summarizerv1pb.TestInferenceModelRequest_builder{
				Model: summarizerv1pb.InferenceModelSpec_builder{
					Openai: summarizerv1pb.OpenAIProvider_builder{
						OpenaiModelId: "gpt-4o",
						BaseUrl:       mockOpenAI.URL,
					}.Build(),
				}.Build(),
				Secret: summarizerv1pb.InferenceSecretSpec_builder{
					Value: "test-api-key",
				}.Build(),
			}.Build(),
			expectSuccess: false,
			// When no client factory is configured, OpenAI provider uses default client
			// which makes a real API call to the mock server, so we expect an authentication error
			messageContains: "Invalid API key",
		},
		{
			name: "unsupported provider type",
			req: summarizerv1pb.TestInferenceModelRequest_builder{
				Model: summarizerv1pb.InferenceModelSpec_builder{}.Build(),
			}.Build(),
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
			assert.Equal(t, tt.expectSuccess, resp.GetSuccess())
			if tt.messageContains != "" {
				assert.Contains(t, resp.GetMessage(), tt.messageContains)
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

// TestServiceUnlicensed uses reflection to verify that every method declared by
// [summarizerv1pb.SummarizerServiceServer] returns a gRPC Unimplemented error
// when [ServiceConfig.IsLicensed] returns false. This ensures that newly added
// endpoints cannot accidentally bypass the entitlement check.
func TestServiceUnlicensed(t *testing.T) {
	t.Parallel()

	newUnlicensedSvc := func(t *testing.T, authorizer authz.Authorizer) *Service {
		t.Helper()
		svc, err := NewService(ServiceConfig{
			Authorizer:        authorizer,
			Backend:           unlicensedStubBackend{},
			Cache:             unlicensedStubCache{},
			SummaryDownloader: unlicensedStubDownloader{},
			Emitter:           eventstest.NewChannelEmitter(0),
			UsageReporter:     &fakeUsageReporter{},
			IsLicensed:        func() bool { return false },
		})
		require.NoError(t, err)
		return svc
	}

	// Most CRUD endpoints require an admin-level context to pass CheckAccessToKind.
	adminCtx, err := authz.NewBuiltinRoleContext(types.RoleAdmin)
	require.NoError(t, err)
	adminSvc := newUnlicensedSvc(t, authz.AuthorizerFunc(func(context.Context) (*authz.Context, error) {
		return adminCtx, nil
	}))

	// IsEnabled requires a proxy builtin role (HasBuiltinRole check) before the
	// license check is reached; use a dedicated service instance for that method.
	proxyCtx, err := authz.NewBuiltinRoleContext(types.RoleProxy)
	require.NoError(t, err)
	proxySvc := newUnlicensedSvc(t, authz.AuthorizerFunc(func(context.Context) (*authz.Context, error) {
		return proxyCtx, nil
	}))

	svcType := reflect.TypeFor[*Service]()
	ctx := t.Context()

	assertUnimplemented := func(t *testing.T, methodName string, results []reflect.Value) {
		t.Helper()
		if methodName == "IsEnabled" {
			require.Len(t, results, 2, "expected 2 return values from %s, got %d", methodName, len(results))
			require.False(t, results[0].IsNil(), "expected non-nil response from %s, got nil", methodName)
			resp := results[0].Interface().(*summarizerv1pb.IsEnabledResponse)
			assert.False(t, resp.GetEnabled(), "expected enabled=false from %s, got enabled=true", methodName)
			return
		}
		require.False(t, results[1].IsNil(), "expected Unimplemented error from %s, got nil", methodName)
		err := results[1].Interface().(error)
		st, ok := status.FromError(err)
		require.True(t, ok, "expected gRPC status error from %s, got: %v", methodName, err)
		require.Equal(t, codes.Unimplemented, st.Code(),
			"expected Unimplemented from %s, got %v: %v", methodName, st.Code(), err)
	}

	tested := 0
	for i := range svcType.NumMethod() {
		m := svcType.Method(i)
		if !m.IsExported() {
			continue
		}

		svcVal := reflect.ValueOf(adminSvc)
		if m.Name == "IsEnabled" {
			svcVal = reflect.ValueOf(proxySvc)
		}
		reqType := m.Type.In(2)
		results := svcVal.MethodByName(m.Name).Call([]reflect.Value{
			reflect.ValueOf(ctx),
			reflect.New(reqType.Elem()),
		})

		t.Run(m.Name, func(t *testing.T) {
			assertUnimplemented(t, m.Name, results)
		})
		tested++
	}
	require.GreaterOrEqual(t, tested, 26, "reflection found fewer methods than expected — interface may have shrunk")
}

type unlicensedStubBackend struct{ services.Summarizer }

type unlicensedStubCache struct {
	services.SummarizerServiceGetter
}

type unlicensedStubDownloader struct{}

func (unlicensedStubDownloader) StreamSessionSummary(context.Context, session.ID) (io.ReadCloser, error) {
	panic("StreamSessionSummary must not be called when unlicensed")
}
