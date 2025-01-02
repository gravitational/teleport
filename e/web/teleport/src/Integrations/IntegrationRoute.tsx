import { Route } from 'teleport/components/Router';
import cfg from 'teleport/config';
import { getRoutesToEnrollIntegrations as getOSSRoutes } from 'teleport/Integrations/Enroll';
import { IntegrationKind } from 'teleport/services/integrations';

import ExternalAuditStorage from './IntegrationEnroll/ExternalAuditStorage';

export function getRoutesToEnrollIntegrations() {
  return [
    ...getOSSRoutes(),
    <Route
      key={IntegrationKind.ExternalAuditStorage}
      exact
      path={cfg.getIntegrationEnrollRoute(IntegrationKind.ExternalAuditStorage)}
      component={ExternalAuditStorage}
    />,
  ];
}
