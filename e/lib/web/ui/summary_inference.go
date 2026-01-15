package ui

import (
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	libslices "github.com/gravitational/teleport/lib/utils/slices"
)

// InferenceModel is a UI representation of an inference model.
type InferenceModel struct {
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Labels      map[string]string   `json:"labels,omitempty"`
	OpenAI      *OpenAIModelConfig  `json:"openai,omitempty"`
	Bedrock     *BedrockModelConfig `json:"bedrock,omitempty"`
}

// OpenAIModelConfig contains OpenAI-specific model configuration.
type OpenAIModelConfig struct {
	// ModelID is the identifier of the OpenAI model.
	ModelID string `json:"modelId"`
	// Temperature is the sampling temperature to use for the model. Optional, defaults the model's default.
	Temperature float64 `json:"temperature,omitempty"`
	// APIKeySecretRef is a reference to an InferenceSecret that contains the
	// OpenAI API key.
	APIKeySecretRef string `json:"api_key_secret_ref,omitempty"`
	// BaseURL is the OpenAI API base URL. Optional, defaults to the public
	// OpenAI API URL. May be used to point to a custom OpenAI-compatible API,
	// such as LiteLLM. In such case, the `api_key_secret_ref` must point to a
	// secret that contains the API key for that custom API.
	BaseURL string `json:"base_url,omitempty"`
}

// BedrockModelConfig contains AWS Bedrock-specific model configuration.
type BedrockModelConfig struct {
	// ModelID is the identifier of the Bedrock model.
	ModelID string `json:"modelId"`
	// Region is the AWS region where the Bedrock model is hosted.
	Region string `json:"region"`
	// Temperature is the sampling temperature to use for the model. Optional, defaults the model's default.
	Temperature float32 `json:"temperature,omitempty"`
	// Integration is the AWS OIDC Integration name. If unset, Teleport will use
	// AWS credentials available on the auth server machine; otherwise, it will
	// use the specified OIDC integration for assuming appropriate role.
	Integration string `json:"integration,omitempty"`
}

// InferenceSecret is a UI representation of an inference secret.
type InferenceSecret struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	// Value is the secret value. It is hidden in UI responses.
	Value string `json:"value,omitempty"`
}

// InferencePolicy is a UI representation of an inference policy.
type InferencePolicy struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Model       string            `json:"model"`
	Kinds       []string          `json:"kinds"`
	Filter      string            `json:"filter,omitempty"`
}

// MakeInferenceModel converts a protobuf InferenceModel to UI representation.
func MakeInferenceModel(model *summarizerv1.InferenceModel) InferenceModel {
	if model == nil {
		return InferenceModel{}
	}

	ui := InferenceModel{
		Name:        model.GetMetadata().GetName(),
		Description: model.GetMetadata().GetDescription(),
		Labels:      model.GetMetadata().GetLabels(),
	}

	spec := model.GetSpec()
	if spec != nil {
		switch provider := spec.GetProvider().(type) {
		case *summarizerv1.InferenceModelSpec_Openai:
			ui.OpenAI = &OpenAIModelConfig{
				ModelID:         provider.Openai.GetOpenaiModelId(),
				Temperature:     provider.Openai.GetTemperature(),
				APIKeySecretRef: provider.Openai.GetApiKeySecretRef(),
				BaseURL:         provider.Openai.GetBaseUrl(),
			}
		case *summarizerv1.InferenceModelSpec_Bedrock:
			ui.Bedrock = &BedrockModelConfig{
				ModelID:     provider.Bedrock.GetBedrockModelId(),
				Region:      provider.Bedrock.GetRegion(),
				Temperature: provider.Bedrock.GetTemperature(),
				Integration: provider.Bedrock.GetIntegration(),
			}
		}
	}

	return ui
}

// MakeInferenceModels converts a slice of protobuf InferenceModels to UI representation.
func MakeInferenceModels(models []*summarizerv1.InferenceModel) []InferenceModel {
	return libslices.Map(models, MakeInferenceModel)
}

// ToProto converts a UI InferenceModel to protobuf representation.
func (m *InferenceModel) ToProto() *summarizerv1.InferenceModel {
	model := &summarizerv1.InferenceModel{
		Kind:    types.KindInferenceModel,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:        m.Name,
			Description: m.Description,
			Labels:      m.Labels,
		},
		Spec: &summarizerv1.InferenceModelSpec{},
	}

	switch {
	case m.OpenAI != nil:
		model.Spec.Provider = &summarizerv1.InferenceModelSpec_Openai{
			Openai: &summarizerv1.OpenAIProvider{
				OpenaiModelId:   m.OpenAI.ModelID,
				Temperature:     m.OpenAI.Temperature,
				ApiKeySecretRef: m.OpenAI.APIKeySecretRef,
				BaseUrl:         m.OpenAI.BaseURL,
			},
		}
	case m.Bedrock != nil:
		model.Spec.Provider = &summarizerv1.InferenceModelSpec_Bedrock{
			Bedrock: &summarizerv1.BedrockProvider{
				BedrockModelId: m.Bedrock.ModelID,
				Region:         m.Bedrock.Region,
				Temperature:    m.Bedrock.Temperature,
				Integration:    m.Bedrock.Integration,
			},
		}
	}

	return model
}

// MakeInferenceSecret converts a protobuf InferenceSecret to UI representation.
func MakeInferenceSecret(secret *summarizerv1.InferenceSecret) InferenceSecret {

	return InferenceSecret{
		Name:        secret.GetMetadata().GetName(),
		Description: secret.GetMetadata().GetDescription(),
		Labels:      secret.GetMetadata().GetLabels(),
		Value:       secret.GetSpec().GetValue(),
	}
}

// MakeInferenceSecrets converts a slice of protobuf InferenceSecrets to UI representation.
func MakeInferenceSecrets(secrets []*summarizerv1.InferenceSecret) []InferenceSecret {
	return libslices.Map(secrets, MakeInferenceSecret)
}

// ToProto converts a UI InferenceSecret to protobuf representation.
func (s *InferenceSecret) ToProto() *summarizerv1.InferenceSecret {
	return &summarizerv1.InferenceSecret{
		Kind:    types.KindInferenceSecret,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:        s.Name,
			Description: s.Description,
			Labels:      s.Labels,
		},
		Spec: &summarizerv1.InferenceSecretSpec{
			Value: s.Value,
		},
	}
}

// MakeInferencePolicy converts a protobuf InferencePolicy to UI representation.
func MakeInferencePolicy(policy *summarizerv1.InferencePolicy) InferencePolicy {
	return InferencePolicy{
		Name:        policy.GetMetadata().GetName(),
		Description: policy.GetMetadata().GetDescription(),
		Labels:      policy.GetMetadata().GetLabels(),
		Model:       policy.GetSpec().GetModel(),
		Kinds:       policy.GetSpec().GetKinds(),
		Filter:      policy.GetSpec().GetFilter(),
	}
}

// MakeInferencePolicies converts a slice of protobuf InferencePolicies to UI representation.
func MakeInferencePolicies(policies []*summarizerv1.InferencePolicy) []InferencePolicy {
	return libslices.Map(policies, MakeInferencePolicy)
}

// ToProto converts a UI InferencePolicy to protobuf representation.
func (p *InferencePolicy) ToProto() *summarizerv1.InferencePolicy {
	return &summarizerv1.InferencePolicy{
		Kind:    types.KindInferencePolicy,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:        p.Name,
			Description: p.Description,
			Labels:      p.Labels,
		},
		Spec: &summarizerv1.InferencePolicySpec{
			Model:  p.Model,
			Kinds:  p.Kinds,
			Filter: p.Filter,
		},
	}
}

// ListInferenceModelsResponse is the response for listing inference models.
type ListInferenceModelsResponse struct {
	// Items is the list of inference models in this page.
	Items []InferenceModel `json:"items"`
	// NextKey is the token to retrieve the next page of results.
	NextKey string `json:"nextKey"`
}

// ListInferenceSecretsResponse is the response for listing inference secrets.
type ListInferenceSecretsResponse struct {
	// Items is the list of inference secrets in this page.
	Items []InferenceSecret `json:"items"`
	// NextKey is the token to retrieve the next page of results.
	NextKey string `json:"nextKey"`
}

// ListInferencePoliciesResponse is the response for listing inference policies.
type ListInferencePoliciesResponse struct {
	// Items is the list of inference policies in this page.
	Items []InferencePolicy `json:"items"`
	// NextKey is the token to retrieve the next page of results.
	NextKey string `json:"nextKey"`
}
