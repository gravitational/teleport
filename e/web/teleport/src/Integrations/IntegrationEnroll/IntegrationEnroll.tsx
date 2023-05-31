import React, { lazy } from 'react';
import { Switch, Route } from 'teleport/components/Router';
import { FeatureBox } from 'teleport/components/Layout';
import { getRoutesToEnrollIntegrations } from 'teleport/Integrations/Enroll';
import cfg from 'teleport/config';

const IntegrationPick = lazy(() => import('./IntegrationPick'));
const PluginEnroll = lazy(() => import('./PluginEnroll'));

export function IntegrationEnroll() {
  return (
    <FeatureBox>
      <Switch>
        {getRoutesToEnrollIntegrations()} {/* eg: enroll aws oidc */}
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
