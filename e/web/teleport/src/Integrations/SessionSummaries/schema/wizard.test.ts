import { TELEPORT_CLOUD_MODEL } from './accessMethods';
import { inferenceWizardSchema, type InferenceWizardForm } from './wizard';

describe('cloud', () => {
  it('validates bedrock integration', () => {
    const input: InferenceWizardForm = {
      isCloud: true,
      kinds: ['ssh', 'k8s'],
      model: 'anthropic.claude-v2',
      modelProvider: 'claude',
      accessMethod: 'bedrock',
      bedrockMode: 'integration',
      integrationName: 'test-integration',
      region: 'us-east-1',
    };

    const result = inferenceWizardSchema.safeParse(input);

    expect(result.success).toBe(true);
  });

  it('validates teleport access method', () => {
    const input: InferenceWizardForm = {
      isCloud: true,
      kinds: ['ssh', 'k8s', 'db'],
      model: TELEPORT_CLOUD_MODEL,
      modelProvider: 'teleport',
      accessMethod: 'teleport',
    };

    const result = inferenceWizardSchema.safeParse(input);

    expect(result.success).toBe(true);
  });

  it('validates openai_api with new key', () => {
    const input: InferenceWizardForm = {
      isCloud: true,
      kinds: ['ssh'],
      model: 'gpt-4',
      modelProvider: 'openai',
      accessMethod: 'openai_api',
      apiKeyMode: 'new',
      apiKey: 'sk-' + 'a'.repeat(50),
    };

    const result = inferenceWizardSchema.safeParse(input);

    expect(result.success).toBe(true);
  });

  it('validates openai_api with existing secret', () => {
    const input: InferenceWizardForm = {
      isCloud: true,
      kinds: ['db'],
      model: 'gpt-4',
      modelProvider: 'openai',
      accessMethod: 'openai_api',
      apiKeyMode: 'existing',
      secretName: 'my-openai-secret',
    };

    const result = inferenceWizardSchema.safeParse(input);

    expect(result.success).toBe(true);
  });

  it('validates openai_compatible with new key', () => {
    const input: InferenceWizardForm = {
      isCloud: true,
      kinds: ['k8s'],
      model: 'llama-3',
      modelProvider: 'other',
      accessMethod: 'openai_compatible',
      apiKeyMode: 'new',
      apiKey: 'some-api-key',
      apiUrl: 'https://api.example.com/v1',
    };

    const result = inferenceWizardSchema.safeParse(input);

    expect(result.success).toBe(true);
  });

  it('validates openai_compatible with existing secret', () => {
    const input: InferenceWizardForm = {
      isCloud: true,
      kinds: ['ssh', 'db'],
      model: 'llama-3',
      modelProvider: 'other',
      accessMethod: 'openai_compatible',
      apiKeyMode: 'existing',
      secretName: 'my-secret',
      apiUrl: 'https://api.example.com/v1',
    };

    const result = inferenceWizardSchema.safeParse(input);

    expect(result.success).toBe(true);
  });
});

describe('self-hosted', () => {
  it('validates bedrock integration', () => {
    const input: InferenceWizardForm = {
      isCloud: false,
      kinds: ['ssh', 'k8s'],
      model: 'anthropic.claude-v2',
      modelProvider: 'claude',
      accessMethod: 'bedrock',
      bedrockMode: 'integration',
      integrationName: 'test-integration',
      region: 'us-west-2',
    };

    const result = inferenceWizardSchema.safeParse(input);

    expect(result.success).toBe(true);
  });

  it('validates bedrock direct mode', () => {
    const input: InferenceWizardForm = {
      isCloud: false,
      kinds: ['ssh'],
      model: 'anthropic.claude-v2',
      modelProvider: 'claude',
      accessMethod: 'bedrock',
      bedrockMode: 'direct',
      region: 'eu-west-1',
    };

    const result = inferenceWizardSchema.safeParse(input);

    expect(result.success).toBe(true);
  });

  it('validates bedrock inference_profile mode', () => {
    const input: InferenceWizardForm = {
      isCloud: false,
      kinds: ['db'],
      model: 'anthropic.claude-v2',
      modelProvider: 'claude',
      accessMethod: 'bedrock',
      bedrockMode: 'inference_profile',
      region: 'ap-northeast-1',
    };

    const result = inferenceWizardSchema.safeParse(input);

    expect(result.success).toBe(true);
  });

  it('validates openai_api with new key', () => {
    const input: InferenceWizardForm = {
      isCloud: false,
      kinds: ['ssh', 'k8s', 'db'],
      model: 'gpt-4-turbo',
      modelProvider: 'openai',
      accessMethod: 'openai_api',
      apiKeyMode: 'new',
      apiKey: 'sk-' + 'x'.repeat(50),
    };

    const result = inferenceWizardSchema.safeParse(input);

    expect(result.success).toBe(true);
  });

  it('validates openai_compatible with new key and optional apiKey', () => {
    const input: InferenceWizardForm = {
      isCloud: false,
      kinds: ['ssh'],
      model: 'local-model',
      modelProvider: 'other',
      accessMethod: 'openai_compatible',
      apiKeyMode: 'new',
      apiUrl: 'http://localhost:8080/v1',
    };

    const result = inferenceWizardSchema.safeParse(input);

    expect(result.success).toBe(true);
  });
});

describe('validation errors', () => {
  it('rejects invalid model provider', () => {
    const input = {
      isCloud: true,
      kinds: ['ssh'],
      model: 'gpt-4',
      modelProvider: 'invalid_provider',
      accessMethod: 'openai_api',
      apiKeyMode: 'existing',
      secretName: 'my-secret',
    };

    const result = inferenceWizardSchema.safeParse(input);

    expect(result.success).toBe(false);
  });
});
