import { StoryObj } from '@storybook/react-vite';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import TeleportEContext from 'e-teleport/teleportContextE';
import cfg from 'teleport/config';
import { ContextProvider } from 'teleport/index';
import type { Plugin } from 'teleport/services/integrations';

import { StatusDetails } from './EntraStatus';
import { entraPlugin, entraPluginErrorStatus } from './fixtures';

export default {
  title: 'TeleportE/Integrations/Status/Entra',
};

export const Default: StoryObj = {
  render: () => {
    const ctx = createTeleportContextE();
    cfg.isPolicyEnabled = true;
    return render(ctx, entraPlugin);
  },
};

export const DirectorySyncError: StoryObj = {
  render: () => {
    const ctx = createTeleportContextE();
    cfg.isPolicyEnabled = true;
    const plugin = { ...entraPlugin };
    plugin.status = entraPluginErrorStatus;
    return render(ctx, plugin);
  },
};

const render = (ctx: TeleportEContext, plugin: Plugin) => {
  return (
    <ContextProvider ctx={ctx}>
      <StatusDetails plugin={plugin} onDelete={() => null} />
    </ContextProvider>
  );
};
