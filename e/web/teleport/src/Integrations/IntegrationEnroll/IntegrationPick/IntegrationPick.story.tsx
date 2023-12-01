import React, { useEffect } from 'react';
import { MemoryRouter } from 'react-router';
import { ContextProvider } from 'teleport';
import {
  IntegrationStatusCode,
  PluginKind,
} from 'teleport/services/integrations';
import { noAccess, allAccessAcl } from 'teleport/mocks/contexts';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import cfg from 'e-teleport/config';
import TeleportEContext from 'e-teleport/teleportContextE';

import { IntegrationPick } from './IntegrationPick';

const { worker, rest } = window.msw;

const onboardSupportPluginKinds: PluginKind[] = [
  'slack',
  'okta',
  'opsgenie',
  'jamf',
];

const defaultIsCloudFlag = cfg.oss.isCloud;
const defaultIsTeam = cfg.oss.isTeam;
const defaultIsEnterprise = cfg.oss.isEnterprise;

export default {
  title: 'TeleportE/Integrations/Picker',
  decorators: [
    Story => {
      cfg.oss.isCloud = true;
      cfg.oss.isEnterprise = true;
      useEffect(() => {
        // Clean up
        return () => {
          cfg.oss.isCloud = defaultIsCloudFlag;
          cfg.oss.isTeam = defaultIsTeam;
          cfg.oss.isEnterprise = defaultIsEnterprise;
        };
      }, []);

      // Reset request handlers added in individual stories.
      worker.resetHandlers();
      return <Story />;
    },
  ],
};

export const NoPluginsEnrolled = () => {
  worker.use(
    rest.get(cfg.api.pluginTypesPath, (req, res, ctx) => {
      return res(ctx.json(onboardSupportPluginKinds));
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

export const PluginsEnrolled = () => {
  worker.use(
    rest.get(cfg.api.pluginTypesPath, (req, res, ctx) => {
      return res(ctx.json(onboardSupportPluginKinds));
    })
  );

  worker.use(
    rest.get(cfg.getPluginUrl(), (req, res, ctx) => {
      return res(ctx.json(mockGetPluginsReply));
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

export const RequiresEnterprise = () => {
  cfg.oss.isTeam = true;
  worker.use(
    rest.get(cfg.api.pluginTypesPath, (req, res, ctx) => {
      return res(ctx.json(onboardSupportPluginKinds));
    })
  );

  worker.use(
    rest.get(cfg.getPluginUrl(), (req, res, ctx) => {
      return res(ctx.json(mockGetPluginsReply));
    })
  );
  const ctx = createTeleportContextE();

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

const mockGetPluginsReply = [
  {
    name: 'plugin-name',
    details: 'some detail',
    type: 'slack',
    statusCode: IntegrationStatusCode.Running,
  },
  {
    name: 'plugin-name2',
    details: 'some detail2',
    type: 'okta',
    statusCode: IntegrationStatusCode.Running,
  },
  {
    name: 'plugin-name3',
    details: 'some detail3',
    type: 'opsgenie',
    statusCode: IntegrationStatusCode.Running,
  },
];
