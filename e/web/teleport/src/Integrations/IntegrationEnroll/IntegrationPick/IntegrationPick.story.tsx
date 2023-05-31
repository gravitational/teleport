import React from 'react';
import { MemoryRouter } from 'react-router';
import { ContextProvider } from 'teleport';
import { IntegrationStatusCode } from 'teleport/services/integrations';
import { noAccess, allAccessAcl } from 'teleport/mocks/contexts';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import cfg from 'e-teleport/config';
import TeleportEContext from 'e-teleport/teleportContextE';

import { IntegrationPick } from './IntegrationPick';

const { worker, rest } = window.msw;

export default {
  title: 'TeleportE/Integrations/Picker',
  decorators: [
    Story => {
      // Reset request handlers added in individual stories.
      worker.resetHandlers();
      return <Story />;
    },
  ],
};

export const NoPluginsEnrolled = () => {
  worker.use(
    rest.get(cfg.api.pluginTypesPath, (req, res, ctx) => {
      return res(ctx.json(['slack']));
    })
  );

  worker.use(
    rest.get(cfg.getPluginUrl(), (req, res, ctx) => {
      return res(ctx.json([]));
    })
  );

  const ctx = createTeleportContextE();
  return render(ctx);
};

export const SlackEnrolled = () => {
  worker.use(
    rest.get(cfg.api.pluginTypesPath, (req, res, ctx) => {
      return res(ctx.json(['slack']));
    })
  );

  worker.use(
    rest.get(cfg.getPluginUrl(), (req, res, ctx) => {
      return res(ctx.json(mockPlugins));
    })
  );

  const ctx = createTeleportContextE();
  return render(ctx);
};

export const Error = () => {
  worker.use(
    rest.get(cfg.api.pluginTypesPath, (req, res, ctx) => {
      return res(ctx.status(500), ctx.json({ message: 'some error message' }));
    })
  );

  const ctx = createTeleportContextE();
  return render(ctx);
};

export const NoAccess = () => {
  const ctx = createTeleportContextE({
    customAcl: {
      ...allAccessAcl,
      plugins: noAccess,
      integrations: { ...noAccess, use: false },
    },
  });

  return render(ctx);
};

function render(ctx: TeleportEContext) {
  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <IntegrationPick />
      </ContextProvider>
    </MemoryRouter>
  );
}

const mockPlugins = [
  {
    name: 'plugin-name',
    details: 'some detail',
    type: 'slack',
    statusCode: IntegrationStatusCode.Running,
  },
];
