import { z } from 'zod';

/**
 * This defines the schema for the credentials part of an inference integration.
 *
 * There are different access methods supported:
 * - OpenAI API with either a new key or an existing secret.
 * - OpenAI Compatible API with either a new key or an existing secret.
 * - AWS Bedrock with either integration mode or self-hosted direct/inference profile mode.
 * - Teleport, for Cloud customers where Teleport provides the model.
 *
 * Each access method has its own specific requirements for the fields needed.
 *
 * The union types `cloudCredentials` and `selfHostedCredentials` combine the relevant schemas
 * for cloud and self-hosted deployments respectively.
 */

/**
 * OpenAI schema
 */

// OpenAI API with a new key (secret will be created)
const openaiApiNewKeySchema = z.object({
  accessMethod: z.literal('openai_api'),
  apiKey: z
    .string()
    .min(50, 'Invalid OpenAI API key')
    .startsWith('sk-', 'OpenAI API keys must start with "sk-"'),
  apiKeyMode: z.literal('new'),
});

// OpenAI API with an existing secret
const openaiApiExistingKeySchema = z.object({
  accessMethod: z.literal('openai_api'),
  apiKeyMode: z.literal('existing'),
  secretName: z.string().min(1, 'Secret name is required'),
});

// OpenAI Compatible API with a new key (secret will be created)
const openaiCompatibleNewKeySchema = z.object({
  accessMethod: z.literal('openai_compatible'),
  apiKey: z.string().optional(),
  apiKeyMode: z.literal('new'),
  apiUrl: z.url('Must be a valid URL').min(1, 'API URL is required'),
});

// OpenAI Compatible API with an existing secret
const openaiCompatibleExistingKeySchema = z.object({
  accessMethod: z.literal('openai_compatible'),
  apiKeyMode: z.literal('existing'),
  apiUrl: z.url('Must be a valid URL').min(1, 'API URL is required'),
  secretName: z.string().min(1, 'Secret name is required'),
});

/**
 * Amazon Bedrock schema
 */

// Base Bedrock schema - region is always required
const bedrockBaseSchema = z.object({
  accessMethod: z.literal('bedrock'),
  region: z.string().min(1, 'Region is required'),
});

// Bedrock using a Teleport AWS integration
const bedrockIntegrationSchema = bedrockBaseSchema.extend({
  bedrockMode: z.literal('integration'),
  integrationName: z.string().min(1, 'Integration name is required'),
});

// Bedrock either directly with credentials on the auth server or using an inference profile (only for self-hosted)
// The `model` field will be used for the value.
const bedrockSelfHostedDirectSchema = bedrockBaseSchema.extend({
  bedrockMode: z.enum(['direct', 'inference_profile']),
});

// Teleport provided credentials (Cloud only)
const teleportCredentialsSchema = z.object({
  accessMethod: z.literal('teleport'),
});

// The allowed credential schemas for cloud
export const cloudCredentials = z.union([
  bedrockIntegrationSchema,
  teleportCredentialsSchema,
  openaiApiNewKeySchema,
  openaiApiExistingKeySchema,
  openaiCompatibleNewKeySchema,
  openaiCompatibleExistingKeySchema,
]);

// The allowed credential schemas for self-hosted
export const selfHostedCredentials = z.union([
  bedrockIntegrationSchema,
  bedrockSelfHostedDirectSchema,
  openaiApiNewKeySchema,
  openaiApiExistingKeySchema,
  openaiCompatibleNewKeySchema,
  openaiCompatibleExistingKeySchema,
]);
