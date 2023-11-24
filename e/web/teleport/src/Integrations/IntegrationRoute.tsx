import React, { lazy } from 'react';

import cfg from 'teleport/config';
import { Route } from 'teleport/components/Router';
import { IntegrationKind } from 'teleport/services/integrations';
import { getRoutesToEnrollIntegrations as getOSSRoutes } from 'teleport/Integrations/Enroll';

const ExternalAuditStorage = lazy(
  () => import('./IntegrationEnroll/ExternalAuditStorage')
);

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
