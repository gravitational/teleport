import { z } from 'zod';

import { cloudCredentials, selfHostedCredentials } from './credentials';
import { inferencePolicySchema } from './policy';
import { ModelProvider } from './types';

/**
 * This defines the schema for the inference setup wizard, where all three steps of creating
 * an inference integration are combined into a single schema (policy, model and secret).
 */

// The wizard will always require the policy and the model provider
const baseFields = inferencePolicySchema.and(
  z.object({
    modelProvider: ModelProvider,
  })
);

// Use `cloud` to discriminate between cloud and self-hosted schemas
const cloudFormSchema = baseFields
  .and(z.object({ isCloud: z.literal(true) }))
  .and(cloudCredentials);

const selfHostedFormSchema = baseFields
  .and(z.object({ isCloud: z.literal(false) }))
  .and(selfHostedCredentials);

export const inferenceWizardSchema = z.union([
  cloudFormSchema,
  selfHostedFormSchema,
]);

export type InferenceWizardForm = z.infer<typeof inferenceWizardSchema>;
