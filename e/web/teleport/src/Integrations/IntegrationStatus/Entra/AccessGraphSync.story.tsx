import { StoryObj } from '@storybook/react-vite';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import cfg from 'teleport/config';
import { ContextProvider } from 'teleport/index';

import { AccessGraphSyncDetails } from './AccessGraphSync';

export default {
  title: 'TeleportE/Integrations/Status/Entra/AccessGraphSync',
};

export const Enabled: StoryObj = {
  render: () => {
    cfg.entitlements.AccessGraph = { enabled: true, limit: 0 };
    return render(true);
  },
};

export const Disabled: StoryObj = {
  render: () => {
    cfg.entitlements.AccessGraph = { enabled: true, limit: 0 };
    return render(false);
  },
};

export const EnabledButMissingLicense: StoryObj = {
  render: () => {
    cfg.entitlements.AccessGraph = { enabled: false, limit: 0 };
    return render(true);
  },
};

export const DisabledAndMissingLicense: StoryObj = {
  render: () => {
    cfg.entitlements.AccessGraph = { enabled: false, limit: 0 };
    return render(false);
  },
};

const render = (accessGraphEnabled: boolean) => {
  return (
    <ContextProvider ctx={createTeleportContextE()}>
      <AccessGraphSyncDetails syncEnabled={accessGraphEnabled} />
    </ContextProvider>
  );
};
