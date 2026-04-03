package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
)

// TestListInferenceResources tests listing inference models, secrets, and policies
// with pagination support.
func TestListInferenceResources(t *testing.T) {
	s := newWebSuite(t, withModules(&modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Policy: {Enabled: true},
				// TODO(emargetis): update after https://github.com/gravitational/teleport/pull/63117 merges
				// ensures that /e does not break when new Access Graph entitlement is added to Teleport
				"AccessGraph": {Enabled: true},
			},
		},
	}))
	authClient := s.newAdminAuthClient(s.ctx, t)
	webPack := s.newAuthWebPack(t, "foo")
	ctx := t.Context()
	clusterName := s.testAuthServer.ClusterName()

	testCases := []struct {
		name           string
		resourceType   string
		createResource func(*testing.T, context.Context, authclient.ClientI, string)
		verifyItem     func(*testing.T, any, string)
	}{
		{
			name:           "InferenceModels",
			resourceType:   "models",
			createResource: createInferenceModel,
			verifyItem: func(t *testing.T, item any, expectedName string) {
				model := item.(ui.InferenceModel)
				require.Equal(t, expectedName, model.Name)
				require.NotNil(t, model.OpenAI)
				require.Equal(t, "gpt-4", model.OpenAI.ModelID)
			},
		},
		{
			name:           "InferenceSecrets",
			resourceType:   "secrets",
			createResource: createInferenceSecret,
			verifyItem: func(t *testing.T, item any, expectedName string) {
				secret := item.(ui.InferenceSecret)
				require.Equal(t, expectedName, secret.Name)
			},
		},
		{
			name:           "InferencePolicies",
			resourceType:   "policies",
			createResource: createInferencePolicy,
			verifyItem: func(t *testing.T, item any, expectedName string) {
				policy := item.(ui.InferencePolicy)
				require.Equal(t, expectedName, policy.Name)
				require.Equal(t, "test-model", policy.Model)
				require.Equal(t, []string{"ssh"}, policy.Kinds)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("Basic", func(t *testing.T) {
				tc.createResource(t, ctx, authClient, "basic-resource-0")

				listEndpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "inference", tc.resourceType)
				resp, err := webPack.clt.Get(ctx, listEndpoint, nil)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, resp.Code())

				items := unmarshalListResponse(t, resp.Bytes(), tc.resourceType)
				require.GreaterOrEqual(t, len(items), 1)
				// Verify our resource is in the list
				found := false
				for _, item := range items {
					var name string
					switch v := item.(type) {
					case ui.InferenceModel:
						name = v.Name
					case ui.InferenceSecret:
						name = v.Name
					case ui.InferencePolicy:
						name = v.Name
					}
					if name == "basic-resource-0" {
						tc.verifyItem(t, item, "basic-resource-0")
						found = true
						break
					}
				}
				require.True(t, found, "resource not found in list")
			})

			t.Run("Paginated", func(t *testing.T) {
				listEndpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "inference", tc.resourceType)

				// Create 10 resources for pagination test
				for i := range 10 {
					tc.createResource(t, ctx, authClient, "paginated-resource-"+strconv.Itoa(i))
				}

				// Test pagination: Get first 5 resources
				queryParams := url.Values{"limit": []string{"5"}}
				resp, err := webPack.clt.Get(ctx, listEndpoint, queryParams)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, resp.Code())

				items := unmarshalListResponse(t, resp.Bytes(), tc.resourceType)
				require.Len(t, items, 5)

				// Get next token
				var nextToken string
				switch tc.resourceType {
				case "models":
					var listResp ui.ListInferenceModelsResponse
					err := json.Unmarshal(resp.Bytes(), &listResp)
					require.NoError(t, err)
					nextToken = listResp.NextKey
				case "secrets":
					var listResp ui.ListInferenceSecretsResponse
					err := json.Unmarshal(resp.Bytes(), &listResp)
					require.NoError(t, err)
					nextToken = listResp.NextKey
				case "policies":
					var listResp ui.ListInferencePoliciesResponse
					err := json.Unmarshal(resp.Bytes(), &listResp)
					require.NoError(t, err)
					nextToken = listResp.NextKey
				}
				require.NotEmpty(t, nextToken, "next token should not be empty when more results exist")

				// Test getting next page
				queryParams = url.Values{
					"limit":    []string{"5"},
					"startKey": []string{nextToken},
				}
				resp, err = webPack.clt.Get(ctx, listEndpoint, queryParams)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, resp.Code())

				items = unmarshalListResponse(t, resp.Bytes(), tc.resourceType)
				require.GreaterOrEqual(t, len(items), 5, "should have at least 5 more items")
			})

			t.Run("Errors", func(t *testing.T) {
				endpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "inference", tc.resourceType)

				errorCases := []struct {
					name           string
					url            url.Values
					badParamErrMsg string
				}{
					{
						name: "invalid limit value",
						url: url.Values{
							"limit":    []string{"invalid"},
							"startKey": []string{""},
						},
						badParamErrMsg: "invalid",
					},
					{
						name: "empty limit should work",
						url: url.Values{
							"startKey": []string{""},
						},
						badParamErrMsg: "",
					},
				}

				for _, ec := range errorCases {
					t.Run(ec.name, func(t *testing.T) {
						resp, err := webPack.clt.Get(ctx, endpoint, ec.url)
						if ec.badParamErrMsg != "" {
							require.ErrorContains(t, err, ec.badParamErrMsg)
						} else {
							require.NoError(t, err)
							items := unmarshalListResponse(t, resp.Bytes(), tc.resourceType)
							require.NotNil(t, items)
						}
					})
				}
			})
		})
	}
}

// TestInferenceModelCRUD tests create, get, update, and delete operations for inference models.
func TestInferenceModelCRUD(t *testing.T) {
	s := newWebSuite(t, withModules(&modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Policy: {Enabled: true},
				// TODO(emargetis): update after https://github.com/gravitational/teleport/pull/63117 merges
				// ensures that /e does not break when new Access Graph entitlement is added to Teleport
				"AccessGraph": {Enabled: true},
			},
		},
	}))
	authClient := s.newAdminAuthClient(s.ctx, t)
	webPack := s.newAuthWebPack(t, "foo")
	ctx := t.Context()
	clusterName := s.testAuthServer.ClusterName()

	t.Run("Create", func(t *testing.T) {
		t.Parallel()
		endpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "inference", "models")
		model := ui.InferenceModel{
			Name:        "test-model",
			Description: "Test model description",
			OpenAI: &ui.OpenAIModelConfig{
				ModelID:         "gpt-4",
				Temperature:     0.7,
				APIKeySecretRef: "my-secret",
			},
		}

		resp, err := webPack.clt.PostJSON(ctx, endpoint, model)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())

		var created ui.InferenceModel
		err = json.Unmarshal(resp.Bytes(), &created)
		require.NoError(t, err)
		require.Equal(t, "test-model", created.Name)
	})

	t.Run("Get", func(t *testing.T) {
		t.Parallel()
		createInferenceModel(t, ctx, authClient, "get-test-model")

		endpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "inference", "models", "get-test-model")
		resp, err := webPack.clt.Get(ctx, endpoint, nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())

		var model ui.InferenceModel
		err = json.Unmarshal(resp.Bytes(), &model)
		require.NoError(t, err)
		require.Equal(t, "get-test-model", model.Name)
	})

	t.Run("Update", func(t *testing.T) {
		t.Parallel()
		createInferenceModel(t, ctx, authClient, "update-test-model")

		endpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "inference", "models", "update-test-model")
		updatedModel := ui.InferenceModel{
			Name:        "update-test-model",
			Description: "Updated description",
			OpenAI: &ui.OpenAIModelConfig{
				ModelID:         "gpt-4-turbo",
				Temperature:     0.9,
				APIKeySecretRef: "new-secret",
			},
		}

		resp, err := webPack.clt.PutJSON(ctx, endpoint, updatedModel)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())

		var result ui.InferenceModel
		err = json.Unmarshal(resp.Bytes(), &result)
		require.NoError(t, err)
		require.Equal(t, "Updated description", result.Description)
		require.Equal(t, "gpt-4-turbo", result.OpenAI.ModelID)
	})

	t.Run("Delete", func(t *testing.T) {
		t.Parallel()
		createInferenceModel(t, ctx, authClient, "delete-test-model")

		endpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "inference", "models", "delete-test-model")
		resp, err := webPack.clt.Delete(ctx, endpoint)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())

		// Verify it's deleted
		_, err = webPack.clt.Get(ctx, endpoint, nil)
		require.Error(t, err)
	})
}

// TestInferenceSecretCRUD tests create, get, update, and delete operations for inference secrets.
func TestInferenceSecretCRUD(t *testing.T) {
	s := newWebSuite(t, withModules(&modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Policy: {Enabled: true},
				// TODO(emargetis): update after https://github.com/gravitational/teleport/pull/63117 merges
				// ensures that /e does not break when new Access Graph entitlement is added to Teleport
				"AccessGraph": {Enabled: true},
			},
		},
	}))
	authClient := s.newAdminAuthClient(s.ctx, t)
	webPack := s.newAuthWebPack(t, "foo")
	ctx := t.Context()
	clusterName := s.testAuthServer.ClusterName()

	t.Run("Create", func(t *testing.T) {
		t.Parallel()
		endpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "inference", "secrets")
		secret := ui.InferenceSecret{
			Name:        "test-secret",
			Description: "Test secret description",
			Value:       "secret-value-123",
		}

		resp, err := webPack.clt.PostJSON(ctx, endpoint, secret)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())

		var created ui.InferenceSecret
		err = json.Unmarshal(resp.Bytes(), &created)
		require.NoError(t, err)
		require.Equal(t, "test-secret", created.Name)
	})

	t.Run("Get", func(t *testing.T) {
		t.Parallel()
		createInferenceSecret(t, ctx, authClient, "get-test-secret")

		endpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "inference", "secrets", "get-test-secret")
		resp, err := webPack.clt.Get(ctx, endpoint, nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())

		var secret ui.InferenceSecret
		err = json.Unmarshal(resp.Bytes(), &secret)
		require.NoError(t, err)
		require.Equal(t, "get-test-secret", secret.Name)
	})

	t.Run("Update", func(t *testing.T) {
		t.Parallel()
		createInferenceSecret(t, ctx, authClient, "update-test-secret")

		endpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "inference", "secrets", "update-test-secret")
		updatedSecret := ui.InferenceSecret{
			Name:        "update-test-secret",
			Description: "Updated secret description",
			Value:       "new-secret-value",
		}

		resp, err := webPack.clt.PutJSON(ctx, endpoint, updatedSecret)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())

		var result ui.InferenceSecret
		err = json.Unmarshal(resp.Bytes(), &result)
		require.NoError(t, err)
		require.Equal(t, "Updated secret description", result.Description)
	})

	t.Run("Delete", func(t *testing.T) {
		t.Parallel()
		createInferenceSecret(t, ctx, authClient, "delete-test-secret")

		endpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "inference", "secrets", "delete-test-secret")
		resp, err := webPack.clt.Delete(ctx, endpoint)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())

		// Verify it's deleted
		_, err = webPack.clt.Get(ctx, endpoint, nil)
		require.Error(t, err)
	})
}

// TestInferencePolicyCRUD tests create, get, update, and delete operations for inference policies.
func TestInferencePolicyCRUD(t *testing.T) {
	s := newWebSuite(t, withModules(&modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Policy: {Enabled: true},
				// TODO(emargetis): update after https://github.com/gravitational/teleport/pull/63117 merges
				// ensures that /e does not break when new Access Graph entitlement is added to Teleport
				"AccessGraph": {Enabled: true},
			},
		},
	}))
	authClient := s.newAdminAuthClient(s.ctx, t)
	webPack := s.newAuthWebPack(t, "foo")
	ctx := t.Context()
	clusterName := s.testAuthServer.ClusterName()

	t.Run("Create", func(t *testing.T) {
		t.Parallel()
		endpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "inference", "policies")
		policy := ui.InferencePolicy{
			Name:        "test-policy",
			Description: "Test policy description",
			Model:       "my-model",
			Kinds:       []string{"ssh", "k8s"},
			Filter:      "equals(resource.metadata.labels[\"env\"], \"prod\")",
		}

		resp, err := webPack.clt.PostJSON(ctx, endpoint, policy)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())

		var created ui.InferencePolicy
		err = json.Unmarshal(resp.Bytes(), &created)
		require.NoError(t, err)
		require.Equal(t, "test-policy", created.Name)
		require.Equal(t, "my-model", created.Model)
	})

	t.Run("Get", func(t *testing.T) {
		t.Parallel()
		createInferencePolicy(t, ctx, authClient, "get-test-policy")

		endpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "inference", "policies", "get-test-policy")
		resp, err := webPack.clt.Get(ctx, endpoint, nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())

		var policy ui.InferencePolicy
		err = json.Unmarshal(resp.Bytes(), &policy)
		require.NoError(t, err)
		require.Equal(t, "get-test-policy", policy.Name)
	})

	t.Run("Update", func(t *testing.T) {
		t.Parallel()
		createInferencePolicy(t, ctx, authClient, "update-test-policy")

		endpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "inference", "policies", "update-test-policy")
		updatedPolicy := ui.InferencePolicy{
			Name:        "update-test-policy",
			Description: "Updated policy description",
			Model:       "new-model",
			Kinds:       []string{"db"},
			Filter:      "equals(resource.metadata.labels[\"env\"], \"prod\")",
		}

		resp, err := webPack.clt.PutJSON(ctx, endpoint, updatedPolicy)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())

		var result ui.InferencePolicy
		err = json.Unmarshal(resp.Bytes(), &result)
		require.NoError(t, err)
		require.Equal(t, "Updated policy description", result.Description)
		require.Equal(t, "new-model", result.Model)
	})

	t.Run("Delete", func(t *testing.T) {
		t.Parallel()
		createInferencePolicy(t, ctx, authClient, "delete-test-policy")

		endpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "inference", "policies", "delete-test-policy")
		resp, err := webPack.clt.Delete(ctx, endpoint)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())

		// Verify it's deleted
		_, err = webPack.clt.Get(ctx, endpoint, nil)
		require.Error(t, err)
	})
}

// Helper functions

func unmarshalListResponse(t *testing.T, data []byte, resourceType string) []any {
	t.Helper()
	switch resourceType {
	case "models":
		var resp ui.ListInferenceModelsResponse
		err := json.Unmarshal(data, &resp)
		require.NoError(t, err)
		items := make([]any, len(resp.Items))
		for i, item := range resp.Items {
			items[i] = item
		}
		return items
	case "secrets":
		var resp ui.ListInferenceSecretsResponse
		err := json.Unmarshal(data, &resp)
		require.NoError(t, err)
		items := make([]any, len(resp.Items))
		for i, item := range resp.Items {
			items[i] = item
		}
		return items
	case "policies":
		var resp ui.ListInferencePoliciesResponse
		err := json.Unmarshal(data, &resp)
		require.NoError(t, err)
		items := make([]any, len(resp.Items))
		for i, item := range resp.Items {
			items[i] = item
		}
		return items
	default:
		t.Fatalf("unknown resource type: %s", resourceType)
		return nil
	}
}

func createInferenceModel(t *testing.T, ctx context.Context, authClient authclient.ClientI, name string) {
	t.Helper()
	_, err := authClient.SummarizerServiceClient().CreateInferenceModel(ctx, &summarizerv1.CreateInferenceModelRequest{
		Model: &summarizerv1.InferenceModel{
			Kind:    types.KindInferenceModel,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name:        name,
				Description: "Test model",
			},
			Spec: &summarizerv1.InferenceModelSpec{
				Provider: &summarizerv1.InferenceModelSpec_Openai{
					Openai: &summarizerv1.OpenAIProvider{
						OpenaiModelId:   "gpt-4",
						Temperature:     0.7,
						ApiKeySecretRef: "test-secret",
					},
				},
			},
		},
	})
	require.NoError(t, err)
}

func createInferenceSecret(t *testing.T, ctx context.Context, authClient authclient.ClientI, name string) {
	t.Helper()
	_, err := authClient.SummarizerServiceClient().CreateInferenceSecret(ctx, &summarizerv1.CreateInferenceSecretRequest{
		Secret: &summarizerv1.InferenceSecret{
			Kind:    types.KindInferenceSecret,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name:        name,
				Description: "Test secret",
			},
			Spec: &summarizerv1.InferenceSecretSpec{
				Value: "test-secret-value",
			},
		},
	})
	require.NoError(t, err)
}

func createInferencePolicy(t *testing.T, ctx context.Context, authClient authclient.ClientI, name string) {
	t.Helper()
	_, err := authClient.SummarizerServiceClient().CreateInferencePolicy(ctx, &summarizerv1.CreateInferencePolicyRequest{
		Policy: &summarizerv1.InferencePolicy{
			Kind:    types.KindInferencePolicy,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name:        name,
				Description: "Test policy",
			},
			Spec: &summarizerv1.InferencePolicySpec{
				Model:  "test-model",
				Kinds:  []string{"ssh"},
				Filter: "",
			},
		},
	})
	require.NoError(t, err)
}

func TestTestInferenceModel(t *testing.T) {
	s := newWebSuite(t, withModules(&modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Policy: {Enabled: true},
				// TODO(emargetis): update after https://github.com/gravitational/teleport/pull/63117 merges
				// ensures that /e does not break when new Access Graph entitlement is added to Teleport
				"AccessGraph": {Enabled: true},
			},
		},
	}))
	webPack := s.newAuthWebPack(t, "foo")
	ctx := t.Context()
	clusterName := s.testAuthServer.ClusterName()

	// Create a mock OpenAI server that returns successful responses
	mockOpenAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": 1234567890,
			"model":   "gpt-4o",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]string{
						"role":    "assistant",
						"content": "test response",
					},
					"finish_reason": "stop",
				},
			},
		})
	}))
	t.Cleanup(mockOpenAI.Close)

	tests := []struct {
		name            string
		req             ui.TestInferenceModelRequest
		expectError     bool
		expectSuccess   bool
		messageContains string
	}{
		{
			name: "missing provider",
			req: ui.TestInferenceModelRequest{
				Secret: "test-secret",
			},
			expectSuccess:   false,
			messageContains: "invalid model spec: missing or unsupported inference provider in spec, supported providers: openai, bedrock",
		},
		{
			name: "OpenAI without secret",
			req: ui.TestInferenceModelRequest{
				OpenAI: &ui.OpenAIModelConfig{
					ModelID: "gpt-4o",
				},
			},
			expectSuccess:   false,
			messageContains: "api_key_secret_ref is required for OpenAI models when no secret is provided in the request",
		},
		{
			name: "OpenAI with valid mock server",
			req: ui.TestInferenceModelRequest{
				OpenAI: &ui.OpenAIModelConfig{
					ModelID: "gpt-4o",
					BaseURL: mockOpenAI.URL,
				},
				Secret: "test-api-key",
			},
			expectSuccess:   true,
			messageContains: "Successfully connected",
		},
	}

	endpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "inference", "test-model")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resp, err := webPack.clt.PostJSON(ctx, endpoint, tt.req)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.Code())

			var result ui.TestInferenceModelResponse
			err = json.Unmarshal(resp.Bytes(), &result)
			require.NoError(t, err)

			require.Equal(t, tt.expectSuccess, result.Success)
			if tt.messageContains != "" {
				require.Contains(t, result.Message, tt.messageContains)
			}
		})
	}
}
