import { z } from 'zod';

import {
  getAccessMethods,
  TELEPORT_CLOUD_MODEL,
} from 'e-teleport/SessionRecordings/setup/schema/accessMethods';

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

const defaultKinds = ['ssh', 'k8s', 'db'] as const;

export function getDefaultInferenceWizardValues(
  modelProvider: ModelProvider,
  isCloud: boolean
): InferenceWizardForm {
  const accessMethods = getAccessMethods(isCloud)[modelProvider];

  if (!accessMethods || accessMethods.length === 0) {
    throw new Error(
      `No access methods available for model provider: ${modelProvider}`
    );
  }

  const base = {
    isCloud,
    kinds: [...defaultKinds],
    model: '',
    modelProvider,
  };

  const defaultAccessMethod = accessMethods[0];

  switch (defaultAccessMethod) {
    case 'bedrock':
      if (isCloud) {
        return {
          ...base,
          accessMethod: 'bedrock',
          bedrockMode: 'integration',
          integrationName: '',
          isCloud: true,
          region: '',
          providedByTeleportCloud: false,
        };
      }

      return {
        ...base,
        accessMethod: 'bedrock',
        bedrockMode: 'integration',
        integrationName: '',
        isCloud: false,
        region: '',
        providedByTeleportCloud: false,
      };

    case 'openai_api':
      return {
        ...base,
        accessMethod: 'openai_api',
        apiKey: '',
        apiKeyMode: 'new',
        providedByTeleportCloud: false,
      };

    case 'openai_compatible':
      return {
        ...base,
        accessMethod: 'openai_compatible',
        apiKey: '',
        apiKeyMode: 'new',
        apiUrl: '',
        providedByTeleportCloud: false,
      };

    case 'teleport':
      if (!isCloud) {
        throw new Error("Teleport's provided model is only available in Cloud");
      }

      return {
        ...base,
        accessMethod: 'teleport',
        isCloud: true,
        model: TELEPORT_CLOUD_MODEL,
        providedByTeleportCloud: true,
        acceptedTerms: false,
      };
  }
}
