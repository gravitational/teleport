import { z } from 'zod';

/**
 * This defines the enums and types that are used across the inference integration schema,
 * to differentiate between different configured options and validate the schema accordingly.
 */

export const ModelProvider = z.enum(['openai', 'claude', 'teleport', 'other']);
export const AccessMethod = z.enum([
  'bedrock',
  'openai_api',
  'openai_compatible',
  'teleport',
]);
export const BedrockMode = z.enum([
  'direct',
  'inference_profile',
  'integration',
]);
export const ApiKeyMode = z.enum(['new', 'existing']);
export const ResourceKind = z.enum(['ssh', 'k8s', 'db']);

export type AccessMethod = z.infer<typeof AccessMethod>;
export type ApiKeyMode = z.infer<typeof ApiKeyMode>;
export type BedrockMode = z.infer<typeof BedrockMode>;
export type ModelProvider = z.infer<typeof ModelProvider>;
