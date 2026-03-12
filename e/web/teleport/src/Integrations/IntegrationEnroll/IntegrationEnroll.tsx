import { getRoutesToEnrollIntegrations } from 'e-teleport/Integrations/IntegrationRoute';
import { FeatureBox } from 'teleport/components/Layout';
import { Route, Switch } from 'teleport/components/Router';
import cfg from 'teleport/config';

import IntegrationPick from './IntegrationPick';
import PluginEnroll from './PluginEnroll';

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
          element={<IntegrationPick />}
        />
        <Route
          key="enroll-plugin"
          path={cfg.routes.integrationEnroll}
          element={<PluginEnroll />}
        />
      </Switch>
    </FeatureBox>
  );
}
