import React, { lazy } from 'react';
import { Switch, Route } from 'teleport/components/Router';
import { FeatureBox } from 'teleport/components/Layout';

import cfg from 'teleport/config';

import { getRoutesToEnrollIntegrations } from 'e-teleport/Integrations/IntegrationRoute';

const IntegrationPick = lazy(() => import('./IntegrationPick'));
const PluginEnroll = lazy(() => import('./PluginEnroll'));

export function IntegrationEnroll() {
  return (
    <FeatureBox>
      <Switch>
        {getRoutesToEnrollIntegrations()}{' '}
        {/* eg: enroll aws oidc, external audit storage */}
        <Route
          key="pick-integration"
          exact
          path={cfg.getIntegrationEnrollRoute()}
          component={IntegrationPick}
        />
        <Route
          key="enroll-plugin"
          exact
          path={cfg.routes.integrationEnroll}
          component={PluginEnroll}
        />
      </Switch>
    </FeatureBox>
  );
}
