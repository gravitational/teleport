import { http, HttpResponse } from 'msw';
import { useEffect } from 'react';

import { Info } from 'design/Alert';

import ecfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import TeleportEContext from 'e-teleport/teleportContextE';
import cfg from 'teleport/config';

import { renderPluginEnroll } from '../../StorybookHelper';

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
        http.get(cfg.api.usersPath, () => {
          return HttpResponse.json([
            { name: 'alice' },
            { name: 'bob' },
            { name: 'carol' },
          ]);
        }),
        http.get(ecfg.api.pluginNeedsCleanupPath, () => {
          return HttpResponse.json({ needsCleanup: false });
        }),
        http.post(ecfg.getPluginValidateUrl(), () => {
          return HttpResponse.json({});
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
