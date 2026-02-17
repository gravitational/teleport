import type { AccessMethod, ModelProvider } from './types';

export const TELEPORT_CLOUD_MODEL = 'teleport-cloud-default';

export function getAccessMethods(
  isCloud: boolean
): Record<ModelProvider, AccessMethod[]> {
  return {
    claude: ['bedrock'],
    openai: ['openai_api', 'bedrock'],
    other: ['openai_compatible', 'bedrock'],
    teleport: isCloud ? ['teleport'] : [], // Teleport providing the model is only available in Cloud
  };
}
