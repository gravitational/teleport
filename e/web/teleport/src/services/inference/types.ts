export interface InferenceModel {
  name: string;
  description?: string;
  labels?: {
    [key: string]: string;
  };
  openai?: OpenAIModelConfig;
  bedrock?: BedrockModelConfig;
}

export interface OpenAIModelConfig {
  modelId: string;
  temperature?: number;
  api_key_secret_ref?: string;
  base_url?: string;
}

export interface BedrockModelConfig {
  modelId: string;
  region: string;
  temperature?: number;
  integration?: string;
}

export interface InferenceSecret {
  name: string;
  description?: string;
  labels?: {
    [key: string]: string;
  };
  value?: string;
}

export interface InferencePolicy {
  name: string;
  description?: string;
  labels?: {
    [key: string]: string;
  };
  model: string;
  kinds: string[];
  filter?: string;
}

export interface TestInferenceModelRequest {
  openai?: OpenAIModelConfig;
  bedrock?: BedrockModelConfig;
  secret?: string;
}

export interface TestInferenceModelResponse {
  success?: boolean;
  message?: string;
}

export interface ListInferenceModelsResponse {
  items?: InferenceModel[];
  nextKey?: string;
}

export interface ListInferenceSecretsResponse {
  items?: InferenceSecret[];
  nextKey?: string;
}

export interface ListInferencePoliciesResponse {
  items?: InferencePolicy[];
  nextKey?: string;
}

export interface ListInferencePoliciesVariables {
  clusterId: string;
  limit?: number;
  startKey?: string;
}

export interface ListInferenceModelsVariables {
  clusterId: string;
  limit?: number;
  startKey?: string;
}

export interface ListInferenceSecretsVariables {
  clusterId: string;
  limit?: number;
  startKey?: string;
}

export interface GetInferenceModelVariables {
  clusterId: string;
  name: string;
}

export interface GetInferencePolicyVariables {
  clusterId: string;
  name: string;
}

export interface GetInferenceSecretVariables {
  clusterId: string;
  name: string;
}

export interface DeleteInferenceModelVariables {
  clusterId: string;
  name: string;
}

export interface DeleteInferencePolicyVariables {
  clusterId: string;
  name: string;
}

export interface DeleteInferenceSecretVariables {
  clusterId: string;
  name: string;
}

export interface CreateInferenceModelVariables {
  clusterId: string;
  model: InferenceModel;
}

export interface CreateInferencePolicyVariables {
  clusterId: string;
  policy: InferencePolicy;
}

export interface CreateInferenceSecretVariables {
  clusterId: string;
  secret: InferenceSecret;
}

export interface UpdateInferenceModelVariables {
  clusterId: string;
  name: string;
  model: InferenceModel;
}

export interface UpdateInferencePolicyVariables {
  clusterId: string;
  name: string;
  policy: InferencePolicy;
}

export interface UpdateInferenceSecretVariables {
  clusterId: string;
  name: string;
  secret: InferenceSecret;
}

export interface TestInferenceModelVariables {
  clusterId: string;
  request: TestInferenceModelRequest;
}
