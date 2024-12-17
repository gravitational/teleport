import { MemoryRouter, Route } from 'react-router';

import cfg from 'teleport/config';

import { pluginMap } from './plugins';
import { PluginEnrollSuccess } from './PluginEnrollSuccess';

import type { CloudHostablePlugin } from 'e-teleport/services/plugins';

export default {
  title: 'TeleportE/Integrations/EnrollSuccess',
};

export const SuccessfullyEnrolledOkta = () => {
  const pathname = cfg.getIntegrationEnrollRoute('okta');
  return (
    <MemoryRouter initialEntries={[pathname]}>
      <Route path={cfg.routes.integrationEnroll}>
        <PluginEnrollSuccess
          plugin={pluginMap['okta'] as CloudHostablePlugin}
          installedPluginName="okta"
        />
        ;
      </Route>
    </MemoryRouter>
  );
};

export const SuccessfullyEnrolledEntraId = () => {
  const pathname = cfg.getIntegrationEnrollRoute('entra-id');
  return (
    <MemoryRouter initialEntries={[pathname]}>
      <Route path={cfg.routes.integrationEnroll}>
        <PluginEnrollSuccess
          plugin={pluginMap['entra-id'] as CloudHostablePlugin}
          installedPluginName="entra"
        />
        ;
      </Route>
    </MemoryRouter>
  );
};
