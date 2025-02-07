import { IntegrationTileSpec } from 'teleport/Integrations/Enroll/IntegrationTiles/integrations';
import { IntegrationKind } from 'teleport/services/integrations';

export const integrationsE: IntegrationTileSpec[] = [
  {
    type: 'integration',
    kind: IntegrationKind.GitHub,
    icon: 'github',
    name: 'GitHub Repository Access',
  },
];
