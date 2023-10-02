import React from 'react';
import { MemoryRouter, Route } from 'react-router';

import cfg from 'teleport/config';

import { pluginMap, HostedPlugin } from './plugins';
import { PluginEnrollSuccess } from './PluginEnrollSuccess';

export default {
  title: 'TeleportE/Integrations/EnrollSuccess',
};

export const SuccessfullyEnrolledOkta = () => {
  const pathname = cfg.getIntegrationEnrollRoute('okta');
  return (
    <MemoryRouter initialEntries={[pathname]}>
      <Route path={cfg.routes.integrationEnroll}>
        <PluginEnrollSuccess plugin={pluginMap['okta'] as HostedPlugin} />;
      </Route>
    </MemoryRouter>
  );
};
