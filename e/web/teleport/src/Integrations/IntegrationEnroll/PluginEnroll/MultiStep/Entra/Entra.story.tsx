import React, { useEffect } from 'react';
import { rest } from 'msw';
import { initialize, mswLoader } from 'msw-storybook-addon';

import { Info } from 'design/Alert';

import cfg from 'teleport/config';

import ecfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import TeleportEContext from 'e-teleport/teleportContextE';

import { renderPluginEnroll } from '../../PluginEnroll.story';

initialize();

const defaultIsEnterprise = cfg.isEnterprise;
const defaultIsPolicyEnabled = cfg.isPolicyEnabled;

const render = (ctx: TeleportEContext) => (
  <>
    <Info>Devs: click "Next" to proceed through the wizard.</Info>
    {renderPluginEnroll('', cfg.getIntegrationEnrollRoute('entra-id'), ctx)}
  </>
);

export default {
  title: 'TeleportE/Integrations/Enroll/Entra',
  loaders: [mswLoader],
  decorators: [
    Story => {
      useEffect(() => {
        // Clean up
        return () => {
          cfg.isEnterprise = defaultIsEnterprise;
          cfg.isPolicyEnabled = defaultIsPolicyEnabled;
        };
      }, []);
      return <Story />;
    },
  ],
  parameters: {
    msw: {
      handlers: [
        rest.get(cfg.api.usersPath, async (_req, res, ctx) => {
          return res(
            ctx.json([{ name: 'alice' }, { name: 'bob' }, { name: 'carol' }])
          );
        }),
        rest.get(ecfg.api.pluginNeedsCleanupPath, async (_req, res, ctx) => {
          return res(ctx.json({ needsCleanup: false }));
        }),
        rest.post(ecfg.getPluginValidateUrl(), async (_req, res, ctx) => {
          return res(ctx.json({}));
        }),
      ],
    },
  },
};

export const Enroll = () => {
  cfg.isEnterprise = true;
  cfg.isPolicyEnabled = false;
  return render(createTeleportContextE());
};

export const EnrollWithPolicy = () => {
  cfg.isEnterprise = true;
  cfg.isPolicyEnabled = true;
  return render(createTeleportContextE());
};
