import { MemoryRouter, Route } from 'react-router';

import cfg from 'teleport/config';

import { ContextProvider } from 'teleport';

import TeleportEContext from 'e-teleport/teleportContextE';

import { PluginEnroll } from './PluginEnroll';

export function renderPluginEnroll(
  search: string,
  pathname = cfg.getIntegrationEnrollRoute('slack'),
  ctx?: TeleportEContext
) {
  return (
    <MemoryRouter initialEntries={[{ pathname, search }]}>
      <Route path={cfg.routes.integrationEnroll}>
        <ContextProvider ctx={ctx}>
          <PluginEnroll />
        </ContextProvider>
      </Route>
    </MemoryRouter>
  );
}
