package summarizerv1

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/summarizer"
	"github.com/gravitational/teleport/lib/auth/authtest"
)

// newTestRetrievalModel creates a test RetrievalModel with OpenAI provider.
func newTestRetrievalModel() *summarizerv1pb.RetrievalModel {
	return summarizer.NewRetrievalModel(summarizerv1pb.RetrievalModelSpec_builder{
		Openai: summarizerv1pb.OpenAIProvider_builder{
			OpenaiModelId:   "text-embedding-3-small",
			ApiKeySecretRef: "secret1",
		}.Build(),
		InferenceModelName: "test-inference-model",
	}.Build())
}

// newTestRetrievalModelBedrock creates a test RetrievalModel with Bedrock provider.
func newTestRetrievalModelBedrock() *summarizerv1pb.RetrievalModel {
	return summarizer.NewRetrievalModel(summarizerv1pb.RetrievalModelSpec_builder{
		Bedrock: summarizerv1pb.BedrockProvider_builder{
			BedrockModelId: "amazon.titan-embed-text-v1",
			Region:         "us-east-1",
		}.Build(),
		InferenceModelName: "test-inference-model",
	}.Build())
}

// createTestInferenceModelForRetrieval creates the inference model referenced
// by the test retrieval models (InferenceModelName: "test-inference-model").
func createTestInferenceModelForRetrieval(ctx context.Context, t *testing.T, sclt summarizerv1pb.SummarizerServiceClient) {
	t.Helper()
	model := newBedrockModel("test-inference-model")
	_, err := sclt.CreateInferenceModel(ctx, summarizerv1pb.CreateInferenceModelRequest_builder{Model: model}.Build())
	require.NoError(t, err)
}

// createTestUserWithRetrieval creates a user that has access to all the configuration
// objects including retrieval model.
func createTestUserWithRetrieval(
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

func TestService_RetrievalModel_CreateAndGet(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	srv := newTestTLSServer(t, withUnrestrictedBedrock())
	user := createTestUserWithRetrieval(t, srv, "test-user")

	clt, err := srv.NewClient(authtest.TestUser(user.GetName()))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()

	secret, _, _ := newTestResources(t, "1")

	// Create the secret first (required by the retrieval model).
	_, err = sclt.CreateInferenceSecret(ctx, summarizerv1pb.CreateInferenceSecretRequest_builder{
		Secret: secret,
	}.Build())
	require.NoError(t, err)

	// Create the inference model referenced by the retrieval model.
	createTestInferenceModelForRetrieval(ctx, t, sclt)

	expectedModel := newTestRetrievalModel()

	// Test creating the retrieval model.
	createdModel, err := sclt.CreateRetrievalModel(ctx, summarizerv1pb.CreateRetrievalModelRequest_builder{
		Model: expectedModel,
	}.Build())
	require.NoError(t, err)
	assertResourceEquals(t, expectedModel, createdModel.GetModel())

	// Test retrieving the retrieval model.
	gotModel, err := sclt.GetRetrievalModel(ctx, &summarizerv1pb.GetRetrievalModelRequest{})
	require.NoError(t, err)
	assertResourceEquals(t, expectedModel, gotModel.GetModel())
}

func TestService_RetrievalModel_Update(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	srv := newTestTLSServer(t, withUnrestrictedBedrock())
	user := createTestUserWithRetrieval(t, srv, "test-user")

	clt, err := srv.NewClient(authtest.TestUser(user.GetName()))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()

	secret, _, _ := newTestResources(t, "1")

	// Create the secret first.
	_, err = sclt.CreateInferenceSecret(ctx, summarizerv1pb.CreateInferenceSecretRequest_builder{
		Secret: secret,
	}.Build())
	require.NoError(t, err)

	// Create the inference model referenced by the retrieval model.
	createTestInferenceModelForRetrieval(ctx, t, sclt)

	expectedModel := newTestRetrievalModel()

	// Create the retrieval model.
	createdModel, err := sclt.CreateRetrievalModel(ctx, summarizerv1pb.CreateRetrievalModelRequest_builder{
		Model: expectedModel,
	}.Build())
	require.NoError(t, err)

	// Test updating the retrieval model.
	createdModel.GetModel().GetSpec().GetOpenai().SetTemperature(0.5)
	updatedModel, err := sclt.UpdateRetrievalModel(ctx, summarizerv1pb.UpdateRetrievalModelRequest_builder{
		Model: proto.Clone(createdModel.GetModel()).(*summarizerv1pb.RetrievalModel),
	}.Build())
	require.NoError(t, err)
	assertResourceEquals(t, createdModel.GetModel(), updatedModel.GetModel())

	gotModel, err := sclt.GetRetrievalModel(ctx, &summarizerv1pb.GetRetrievalModelRequest{})
	require.NoError(t, err)
	assertResourceEquals(t, createdModel.GetModel(), gotModel.GetModel())
}

func TestService_RetrievalModel_Upsert(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	srv := newTestTLSServer(t, withUnrestrictedBedrock())
	user := createTestUserWithRetrieval(t, srv, "test-user")

	clt, err := srv.NewClient(authtest.TestUser(user.GetName()))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()

	secret, _, _ := newTestResources(t, "1")

	// Create the secret first.
	_, err = sclt.CreateInferenceSecret(ctx, summarizerv1pb.CreateInferenceSecretRequest_builder{
		Secret: secret,
	}.Build())
	require.NoError(t, err)

	// Create the inference model referenced by the retrieval model.
	createTestInferenceModelForRetrieval(ctx, t, sclt)

	expectedModel := newTestRetrievalModel()

	// Test creating by upserting the retrieval model.
	createdModel, err := sclt.UpsertRetrievalModel(ctx, summarizerv1pb.UpsertRetrievalModelRequest_builder{
		Model: expectedModel,
	}.Build())
	require.NoError(t, err)
	assertResourceEquals(t, expectedModel, createdModel.GetModel())

	// Test updating the retrieval model.
	createdModel.GetModel().GetSpec().GetOpenai().SetTemperature(0.8)
	updatedModel, err := sclt.UpsertRetrievalModel(ctx, summarizerv1pb.UpsertRetrievalModelRequest_builder{
		Model: proto.Clone(createdModel.GetModel()).(*summarizerv1pb.RetrievalModel),
	}.Build())
	require.NoError(t, err)
	assertResourceEquals(t, createdModel.GetModel(), updatedModel.GetModel())

	gotModel, err := sclt.GetRetrievalModel(ctx, &summarizerv1pb.GetRetrievalModelRequest{})
	require.NoError(t, err)
	assertResourceEquals(t, createdModel.GetModel(), gotModel.GetModel())
}

func TestService_RetrievalModel_Delete(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	srv := newTestTLSServer(t, withUnrestrictedBedrock())
	user := createTestUserWithRetrieval(t, srv, "test-user")

	clt, err := srv.NewClient(authtest.TestUser(user.GetName()))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()

	secret, _, _ := newTestResources(t, "1")

	// Create the secret first.
	_, err = sclt.CreateInferenceSecret(ctx, summarizerv1pb.CreateInferenceSecretRequest_builder{
		Secret: secret,
	}.Build())
	require.NoError(t, err)

	// Create the inference model referenced by the retrieval model.
	createTestInferenceModelForRetrieval(ctx, t, sclt)

	expectedModel := newTestRetrievalModel()

	// Create the retrieval model.
	_, err = sclt.CreateRetrievalModel(ctx, summarizerv1pb.CreateRetrievalModelRequest_builder{
		Model: expectedModel,
	}.Build())
	require.NoError(t, err)

	// Delete the retrieval model.
	_, err = sclt.DeleteRetrievalModel(ctx, &summarizerv1pb.DeleteRetrievalModelRequest{})
	require.NoError(t, err)

	// Verify that the retrieval model is deleted.
	_, err = sclt.GetRetrievalModel(ctx, &summarizerv1pb.GetRetrievalModelRequest{})
	require.Error(t, err)
	assert.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)
}

func TestService_RetrievalModel_RBAC(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	srv := newTestTLSServer(t, withUnrestrictedBedrock())
	admin := createTestUserWithRetrieval(t, srv, "admin")

	// Set up resources.
	clt, err := srv.NewClient(authtest.TestUser(admin.GetName()))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()

	secret, _, _ := newTestResources(t, "1")

	// Create the secret first.
	_, err = sclt.CreateInferenceSecret(ctx, summarizerv1pb.CreateInferenceSecretRequest_builder{
		Secret: secret,
	}.Build())
	require.NoError(t, err)

	// Create the inference model referenced by the retrieval model.
	createTestInferenceModelForRetrieval(ctx, t, sclt)

	retrievalModel := newTestRetrievalModel()
	createdModel, err := sclt.CreateRetrievalModel(ctx, summarizerv1pb.CreateRetrievalModelRequest_builder{
		Model: retrievalModel,
	}.Build())
	require.NoError(t, err)

	// Resources for further creation attempts.
	retrievalModel2 := newTestRetrievalModelBedrock()

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
			name:     "create retrieval model",
			resource: types.KindRetrievalModel,
			verbs:    []string{types.VerbCreate},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.CreateRetrievalModel(ctx, summarizerv1pb.CreateRetrievalModelRequest_builder{
					Model: retrievalModel2,
				}.Build())
				return err
			},
		},
		{
			name:     "get retrieval model",
			resource: types.KindRetrievalModel,
			verbs:    []string{types.VerbRead},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.GetRetrievalModel(ctx, &summarizerv1pb.GetRetrievalModelRequest{})
				return err
			},
		},
		{
			name:     "update retrieval model",
			resource: types.KindRetrievalModel,
			verbs:    []string{types.VerbUpdate},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.UpdateRetrievalModel(ctx, summarizerv1pb.UpdateRetrievalModelRequest_builder{
					Model: createdModel.GetModel(),
				}.Build())
				return err
			},
		},
		{
			name:     "upsert retrieval model",
			resource: types.KindRetrievalModel,
			verbs:    []string{types.VerbCreate, types.VerbUpdate},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.UpsertRetrievalModel(ctx, summarizerv1pb.UpsertRetrievalModelRequest_builder{
					Model: createdModel.GetModel(),
				}.Build())
				return err
			},
		},
		{
			name:     "delete retrieval model",
			resource: types.KindRetrievalModel,
			verbs:    []string{types.VerbDelete},
			fn: func(ctx context.Context, sclt summarizerv1pb.SummarizerServiceClient) error {
				_, err := sclt.DeleteRetrievalModel(ctx, &summarizerv1pb.DeleteRetrievalModelRequest{})
				return err
			},
		},
	}

	for i, tc := range cases {
		for j, verb := range tc.verbs {
			t.Run(fmt.Sprintf("%s (%s)", tc.name, verb), func(t *testing.T) {
				user := createTestUserWithRetrieval(
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

func TestService_RetrievalModel_SingletonBehavior(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	srv := newTestTLSServer(t, withUnrestrictedBedrock())
	user := createTestUserWithRetrieval(t, srv, "test-user")

	clt, err := srv.NewClient(authtest.TestUser(user.GetName()))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()

	secret, _, _ := newTestResources(t, "1")

	// Create the secret first.
	_, err = sclt.CreateInferenceSecret(ctx, summarizerv1pb.CreateInferenceSecretRequest_builder{
		Secret: secret,
	}.Build())
	require.NoError(t, err)

	// Create the inference model referenced by the retrieval model.
	createTestInferenceModelForRetrieval(ctx, t, sclt)

	model1 := newTestRetrievalModel()

	// Create the first retrieval model.
	_, err = sclt.CreateRetrievalModel(ctx, summarizerv1pb.CreateRetrievalModelRequest_builder{
		Model: model1,
	}.Build())
	require.NoError(t, err)

	// Attempt to create a second retrieval model should fail
	// (only one retrieval model can exist per cluster).
	model2 := newTestRetrievalModelBedrock()
	_, err = sclt.CreateRetrievalModel(ctx, summarizerv1pb.CreateRetrievalModelRequest_builder{
		Model: model2,
	}.Build())
	require.Error(t, err)
	assert.True(t, trace.IsAlreadyExists(err), "expected AlreadyExists error, got %v", err)

	// Verify that the original model is still there.
	gotModel, err := sclt.GetRetrievalModel(ctx, &summarizerv1pb.GetRetrievalModelRequest{})
	require.NoError(t, err)
	assertResourceEquals(t, model1, gotModel.GetModel())
}

func TestService_RetrievalModel_ProviderVariations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		model *summarizerv1pb.RetrievalModel
	}{
		{
			name:  "OpenAI provider",
			model: newTestRetrievalModel(),
		},
		{
			name:  "Bedrock provider",
			model: newTestRetrievalModelBedrock(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			srv := newTestTLSServer(t, withUnrestrictedBedrock())
			user := createTestUserWithRetrieval(t, srv, "test-user")

			clt, err := srv.NewClient(authtest.TestUser(user.GetName()))
			require.NoError(t, err)
			sclt := clt.SummarizerServiceClient()

			secret, _, _ := newTestResources(t, "1")

			// Create the secret first.
			_, err = sclt.CreateInferenceSecret(ctx, summarizerv1pb.CreateInferenceSecretRequest_builder{
				Secret: secret,
			}.Build())
			require.NoError(t, err)

			// Create the inference model referenced by the retrieval model.
			createTestInferenceModelForRetrieval(ctx, t, sclt)

			// Create the retrieval model.
			createdModel, err := sclt.CreateRetrievalModel(ctx, summarizerv1pb.CreateRetrievalModelRequest_builder{
				Model: tt.model,
			}.Build())
			require.NoError(t, err)
			assertResourceEquals(t, tt.model, createdModel.GetModel())

			// Retrieve and verify the model.
			gotModel, err := sclt.GetRetrievalModel(ctx, &summarizerv1pb.GetRetrievalModelRequest{})
			require.NoError(t, err)
			assertResourceEquals(t, tt.model, gotModel.GetModel())
		})
	}
}

func TestService_RetrievalModel_UpdateProvider(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	srv := newTestTLSServer(t, withUnrestrictedBedrock())
	user := createTestUserWithRetrieval(t, srv, "test-user")

	clt, err := srv.NewClient(authtest.TestUser(user.GetName()))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()

	secret, _, _ := newTestResources(t, "1")

	// Create the secret first.
	_, err = sclt.CreateInferenceSecret(ctx, summarizerv1pb.CreateInferenceSecretRequest_builder{
		Secret: secret,
	}.Build())
	require.NoError(t, err)

	// Create the inference model referenced by the retrieval model.
	createTestInferenceModelForRetrieval(ctx, t, sclt)

	// Create an retrieval model with OpenAI provider.
	openaiModel := newTestRetrievalModel()
	createdModel, err := sclt.CreateRetrievalModel(ctx, summarizerv1pb.CreateRetrievalModelRequest_builder{
		Model: openaiModel,
	}.Build())
	require.NoError(t, err)

	// Update to use Bedrock provider instead.
	bedrockModel := newTestRetrievalModelBedrock()
	// Copy the metadata from the created model to preserve the revision.
	bedrockModel.SetMetadata(createdModel.GetModel().GetMetadata())
	updatedModel, err := sclt.UpdateRetrievalModel(ctx, summarizerv1pb.UpdateRetrievalModelRequest_builder{
		Model: bedrockModel,
	}.Build())
	require.NoError(t, err)
	assertResourceEquals(t, bedrockModel, updatedModel.GetModel())

	// Verify the update persisted.
	gotModel, err := sclt.GetRetrievalModel(ctx, &summarizerv1pb.GetRetrievalModelRequest{})
	require.NoError(t, err)
	assertResourceEquals(t, bedrockModel, gotModel.GetModel())
	assert.Empty(t, cmp.Diff(
		bedrockModel.GetSpec().GetBedrock(),
		gotModel.GetModel().GetSpec().GetBedrock(),
		protocmp.Transform(),
	))
}

func TestService_RetrievalModel_InferenceModelExistenceValidation(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	srv := newTestTLSServer(t, withUnrestrictedBedrock())
	user := createTestUserWithRetrieval(t, srv, "test-user")

	clt, err := srv.NewClient(authtest.TestUser(user.GetName()))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()

	secret, _, _ := newTestResources(t, "1")
	_, err = sclt.CreateInferenceSecret(ctx, summarizerv1pb.CreateInferenceSecretRequest_builder{
		Secret: secret,
	}.Build())
	require.NoError(t, err)

	// Attempts to create, update, and upsert a retrieval model that references
	// a non-existent inference model should all fail.
	model := newTestRetrievalModel() // references "test-inference-model"

	_, err = sclt.CreateRetrievalModel(ctx, summarizerv1pb.CreateRetrievalModelRequest_builder{
		Model: model,
	}.Build())
	require.Error(t, err, "creating a retrieval model with non-existent inference model should fail")

	_, err = sclt.UpdateRetrievalModel(ctx, summarizerv1pb.UpdateRetrievalModelRequest_builder{
		Model: model,
	}.Build())
	require.Error(t, err, "updating a retrieval model with non-existent inference model should fail")

	_, err = sclt.UpsertRetrievalModel(ctx, summarizerv1pb.UpsertRetrievalModelRequest_builder{
		Model: model,
	}.Build())
	require.Error(t, err, "upserting a retrieval model with non-existent inference model should fail")

	// Once the inference model exists, creation should succeed.
	createTestInferenceModelForRetrieval(ctx, t, sclt)

	createdModel, err := sclt.CreateRetrievalModel(ctx, summarizerv1pb.CreateRetrievalModelRequest_builder{
		Model: model,
	}.Build())
	require.NoError(t, err)
	assertResourceEquals(t, model, createdModel.GetModel())
}

func TestService_RetrievalModel_RestrictedBedrock(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	srv := newTestTLSServer(t)
	user := createTestUserWithRetrieval(t, srv, "test-user")

	clt, err := srv.NewClient(authtest.TestUser(user.GetName()))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()

	secret, _, _ := newTestResources(t, "1")

	// Create the secret first (required by the retrieval model).
	_, err = sclt.CreateInferenceSecret(ctx, summarizerv1pb.CreateInferenceSecretRequest_builder{
		Secret: secret,
	}.Build())
	require.NoError(t, err)

	// Create the inference model referenced by the retrieval model.
	createTestInferenceModelForRetrieval(ctx, t, sclt)

	expectedModel := newTestRetrievalModelBedrock()

	// Test creating the retrieval model.
	_, err = sclt.CreateRetrievalModel(ctx, summarizerv1pb.CreateRetrievalModelRequest_builder{
		Model: expectedModel,
	}.Build())
	require.Error(t, err)
}

func TestService_TestRetrievalModel(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Mock OpenAI server that always returns 401 Unauthorized to simulate an
	// invalid API key.
	mockOpenAIUnauthorized := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": {"message": "Invalid API key"}}`))
	}))
	t.Cleanup(mockOpenAIUnauthorized.Close)

	// Mock OpenAI server that returns a valid embeddings response.
	mockOpenAISuccess := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"object": "list",
			"data": [{"object": "embedding", "index": 0, "embedding": [0.1, 0.2, 0.3]}],
			"model": "text-embedding-3-small",
			"usage": {"prompt_tokens": 1, "total_tokens": 1}
		}`))
	}))
	t.Cleanup(mockOpenAISuccess.Close)

	srv := newTestTLSServer(t, withUnrestrictedBedrock())
	user := createTestUserWithRetrieval(t, srv, "test-user")

	clt, err := srv.NewClient(authtest.TestUser(user.GetName()))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()

	// Store a secret in the backend for the api_key_secret_ref tests.
	storedSecret := summarizer.NewInferenceSecret("stored-secret", summarizerv1pb.InferenceSecretSpec_builder{
		Value: "stored-api-key",
	}.Build())
	_, err = sclt.CreateInferenceSecret(ctx, summarizerv1pb.CreateInferenceSecretRequest_builder{
		Secret: storedSecret,
	}.Build())
	require.NoError(t, err)

	tests := []struct {
		name            string
		req             *summarizerv1pb.TestRetrievalModelRequest
		expectError     bool
		expectSuccess   bool
		messageContains string
	}{
		{
			name: "nil model spec",
			req: summarizerv1pb.TestRetrievalModelRequest_builder{
				Model: nil,
			}.Build(),
			expectSuccess:   false,
			messageContains: "model spec is required",
		},
		{
			name: "invalid model spec - no embeddings provider",
			req: summarizerv1pb.TestRetrievalModelRequest_builder{
				Model: summarizerv1pb.RetrievalModelSpec_builder{
					InferenceModelName: "some-model",
				}.Build(),
			}.Build(),
			expectSuccess:   false,
			messageContains: "invalid model spec",
		},
		{
			// Validation requires api_key_secret_ref for OpenAI models; the
			// error comes from ValidateRetrievalModel before reaching provider logic.
			name: "OpenAI without api_key_secret_ref",
			req: summarizerv1pb.TestRetrievalModelRequest_builder{
				Model: summarizerv1pb.RetrievalModelSpec_builder{
					Openai: summarizerv1pb.OpenAIProvider_builder{
						OpenaiModelId: "text-embedding-3-small",
					}.Build(),
					InferenceModelName: "test-inference-model",
				}.Build(),
			}.Build(),
			expectSuccess:   false,
			messageContains: "api_key_secret_ref is required",
		},
		{
			// api_key_secret_ref is set but the secret does not exist in the
			// backend and no inline secret was provided.
			name: "OpenAI with api_key_secret_ref pointing to non-existent secret",
			req: summarizerv1pb.TestRetrievalModelRequest_builder{
				Model: summarizerv1pb.RetrievalModelSpec_builder{
					Openai: summarizerv1pb.OpenAIProvider_builder{
						OpenaiModelId:   "text-embedding-3-small",
						ApiKeySecretRef: "non-existent-secret",
					}.Build(),
					InferenceModelName: "test-inference-model",
				}.Build(),
			}.Build(),
			expectSuccess:   false,
			messageContains: "not found in backend",
		},
		{
			// Inline secret overrides api_key_secret_ref lookup; the mock server
			// returns 401 to simulate an invalid API key.
			name: "OpenAI with inline secret and invalid API key",
			req: summarizerv1pb.TestRetrievalModelRequest_builder{
				Model: summarizerv1pb.RetrievalModelSpec_builder{
					Openai: summarizerv1pb.OpenAIProvider_builder{
						OpenaiModelId:   "text-embedding-3-small",
						ApiKeySecretRef: "some-secret",
						BaseUrl:         mockOpenAIUnauthorized.URL,
					}.Build(),
					InferenceModelName: "test-inference-model",
				}.Build(),
				Secret: summarizerv1pb.InferenceSecretSpec_builder{
					Value: "invalid-api-key",
				}.Build(),
			}.Build(),
			expectSuccess:   false,
			messageContains: "Invalid API key",
		},
		{
			// No inline secret; the provider fetches from the backend using
			// api_key_secret_ref. The stored secret exists but the mock server
			// returns 401.
			name: "OpenAI with stored secret and invalid API key",
			req: summarizerv1pb.TestRetrievalModelRequest_builder{
				Model: summarizerv1pb.RetrievalModelSpec_builder{
					Openai: summarizerv1pb.OpenAIProvider_builder{
						OpenaiModelId:   "text-embedding-3-small",
						ApiKeySecretRef: "stored-secret",
						BaseUrl:         mockOpenAIUnauthorized.URL,
					}.Build(),
					InferenceModelName: "test-inference-model",
				}.Build(),
			}.Build(),
			expectSuccess:   false,
			messageContains: "Invalid API key",
		},
		{
			// Inline secret with a mock server that returns a valid embeddings
			// response — the full round-trip should succeed.
			name: "OpenAI with inline secret and successful response",
			req: summarizerv1pb.TestRetrievalModelRequest_builder{
				Model: summarizerv1pb.RetrievalModelSpec_builder{
					Openai: summarizerv1pb.OpenAIProvider_builder{
						OpenaiModelId:   "text-embedding-3-small",
						ApiKeySecretRef: "some-secret",
						BaseUrl:         mockOpenAISuccess.URL,
					}.Build(),
					InferenceModelName: "test-inference-model",
				}.Build(),
				Secret: summarizerv1pb.InferenceSecretSpec_builder{
					Value: "valid-api-key",
				}.Build(),
			}.Build(),
			expectSuccess:   true,
			messageContains: "Successfully connected",
		},
		{
			// withUnrestrictedBedrock() is set on this server so the Bedrock
			// restriction check passes. The call will still fail because there
			// are no real AWS credentials, but the failure message should NOT
			// be the "restricted" message.
			name: "Bedrock unrestricted but no real AWS creds",
			req: summarizerv1pb.TestRetrievalModelRequest_builder{
				Model: summarizerv1pb.RetrievalModelSpec_builder{
					Bedrock: summarizerv1pb.BedrockProvider_builder{
						BedrockModelId: "amazon.titan-embed-text-v1",
						Region:         "us-east-1",
					}.Build(),
					InferenceModelName: "test-inference-model",
				}.Build(),
			}.Build(),
			expectSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := sclt.TestRetrievalModel(ctx, tt.req)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, resp)
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

func TestService_TestRetrievalModel_RBAC(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	srv := newTestTLSServer(t)

	cases := []struct {
		name string
		verb string
	}{
		{name: "missing create verb", verb: types.VerbCreate},
		{name: "missing update verb", verb: types.VerbUpdate},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			user := createTestUserWithRetrieval(
				t, srv, fmt.Sprintf("rbac-test-user-%d", i+1),
				authtest.WithUserMutator(func(user types.User) {
					roleName := "deny-" + user.GetName()
					role, err := types.NewRole(roleName, types.RoleSpecV6{
						Deny: types.RoleConditions{
							Rules: []types.Rule{
								types.NewRule(types.KindRetrievalModel, []string{tc.verb}),
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

			_, err = sclt.TestRetrievalModel(ctx, summarizerv1pb.TestRetrievalModelRequest_builder{
				Model: summarizerv1pb.RetrievalModelSpec_builder{
					Openai: summarizerv1pb.OpenAIProvider_builder{
						OpenaiModelId:   "text-embedding-3-small",
						ApiKeySecretRef: "some-secret",
					}.Build(),
					InferenceModelName: "test-inference-model",
				}.Build(),
			}.Build())
			require.Error(t, err)
			assert.True(t, trace.IsAccessDenied(err), "expected AccessDenied, got %v", err)
		})
	}
}

func TestService_TestRetrievalModel_RestrictedBedrock(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Server WITHOUT unrestricted Bedrock (simulates Teleport Cloud restriction).
	srv := newTestTLSServer(t)
	user := createTestUserWithRetrieval(t, srv, "test-user")

	clt, err := srv.NewClient(authtest.TestUser(user.GetName()))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()

	resp, err := sclt.TestRetrievalModel(ctx, summarizerv1pb.TestRetrievalModelRequest_builder{
		Model: summarizerv1pb.RetrievalModelSpec_builder{
			Bedrock: summarizerv1pb.BedrockProvider_builder{
				BedrockModelId: "amazon.titan-embed-text-v1",
				Region:         "us-east-1",
			}.Build(),
			InferenceModelName: "test-inference-model",
		}.Build(),
	}.Build())
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.GetSuccess())
	assert.Contains(t, resp.GetMessage(), "access to Amazon Bedrock models provided by Teleport Cloud is restricted")
}
