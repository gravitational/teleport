import { IntegrationTileSpec } from 'teleport/Integrations/Enroll/IntegrationTiles/integrations';
import { IntegrationKind } from 'teleport/services/integrations';

export const integrationsE: IntegrationTileSpec[] = [
  {
    type: 'integration',
    description:
      'Proxy Git commands and use short-lived SSH certificates to authenticate with GitHub.',
    kind: IntegrationKind.GitHub,
    icon: 'github',
    name: 'GitHub Repository Access',
    tags: ['resourceaccess'],
  },
];
