import { StoryObj } from '@storybook/react-vite';

import cfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import TeleportEContext from 'e-teleport/teleportContextE';
import { ContextProvider } from 'teleport/index';
import { allAccessAcl, noAccess } from 'teleport/mocks/contexts';

import { SsoDetails } from './Sso';

export default {
  title: 'TeleportE/Integrations/Status/Entra/Sso',
};

export const Default: StoryObj = {
  render: () => {
    return render(createTeleportContextE(), 'entra-id');
  },
};

export const NoSSOConnectorAccess: StoryObj = {
  render: () => {
    cfg.oss.entitlements.Identity = { enabled: true, limit: 0 };
    const ctx = createTeleportContextE({
      customAcl: {
        ...allAccessAcl,
        authConnectors: noAccess,
      },
    });
    return render(ctx, 'entra-id');
  },
};

const render = (ctx: TeleportEContext, connectorName: string) => {
  return (
    <ContextProvider ctx={ctx}>
      <SsoDetails connectorName={connectorName} />
    </ContextProvider>
  );
};
