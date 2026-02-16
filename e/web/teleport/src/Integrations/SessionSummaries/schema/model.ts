import { z } from 'zod';

import type { InferenceModel } from 'e-teleport/services/inference';

import { cloudCredentials, selfHostedCredentials } from './credentials';

const baseFields = z.object({
  model: z.string().min(1, 'Model is required'),
});

const cloudFormCredentials = baseFields
  .extend({ cloud: z.literal(true) })
  .and(cloudCredentials);

const selfHostedFormCredentials = baseFields
  .extend({ cloud: z.literal(false) })
  .and(selfHostedCredentials);

export const modelSchema = z.union([
  cloudFormCredentials,
  selfHostedFormCredentials,
]);

export type InferenceModelSchema = z.infer<typeof modelSchema>;

export function convertModelSchemaToApi(
  name: string,
  values: InferenceModelSchema
): InferenceModel {
  switch (values.accessMethod) {
    case 'openai_compatible':
      if (values.apiKeyMode === 'new') {
        throw new Error('Only existing secrets are supported');
      }

      return {
        name,
        openai: {
          base_url: values.apiUrl,
          api_key_secret_ref: values.secretName,
          modelId: values.model,
        },
      };

    case 'openai_api':
      if (values.apiKeyMode === 'new') {
        throw new Error('Only existing secrets are supported');
      }

      return {
        name,
        openai: {
          modelId: values.model,
          api_key_secret_ref: values.secretName,
        },
      };

    case 'bedrock':
      if (values.bedrockMode === 'integration') {
        return {
          name,
          bedrock: {
            modelId: values.model,
            region: values.region,
            integration: values.integrationName,
          },
        };
      } else {
        return {
          name,
          bedrock: {
            modelId: values.model,
            region: values.region,
          },
        };
      }
  }

  throw new Error('Unsupported access method');
}
