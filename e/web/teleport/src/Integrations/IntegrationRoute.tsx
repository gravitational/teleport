import { Route } from 'teleport/components/Router';
import cfg from 'teleport/config';
import { getRoutesToEnrollIntegrations as getOSSRoutes } from 'teleport/Integrations/Enroll';
import { IntegrationKind } from 'teleport/services/integrations';

import ExternalAuditStorage from './IntegrationEnroll/ExternalAuditStorage';
import { GitHub } from './IntegrationEnroll/PluginEnroll/MultiStep/GitHub';

export function getRoutesToEnrollIntegrations() {
  return [
    ...getOSSRoutes(),
    <Route
      key={IntegrationKind.ExternalAuditStorage}
      exact
      path={cfg.getIntegrationEnrollRoute(IntegrationKind.ExternalAuditStorage)}
      element={<ExternalAuditStorage />}
    />,
    <Route
      key={IntegrationKind.GitHub}
      exact
      path={cfg.getIntegrationEnrollRoute(IntegrationKind.GitHub)}
      element={<GitHub />}
    />,
  ];
}
