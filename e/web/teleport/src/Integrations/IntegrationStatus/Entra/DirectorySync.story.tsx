import { StoryObj } from '@storybook/react-vite';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import cfg from 'teleport/config';
import { ContextProvider } from 'teleport/index';
import type {
  PluginEntraIdSpec,
  PluginEntraIDStatusDetails,
  PluginStatus,
} from 'teleport/services/integrations';

import { DirectorySyncDetails } from './DirectorySync';
import { entraPlugin, entraPluginErrorStatus } from './fixtures';

export default {
  title: 'TeleportE/Integrations/Status/Entra/DirectorySync',
};

export const Default: StoryObj = {
  render: () => {
    return render(entraPlugin.spec, entraPlugin.status);
  },
};

export const NoSettings: StoryObj = {
  render: () => {
    const spec = { ...entraPlugin.spec };
    spec.defaultOwners = [];
    spec.groupFilters = {
      id: [],
      nameRegex: [],
      excludeId: [],
      excludeNameRegex: [],
    };

    return render(spec, entraPlugin.status);
  },
};

export const ZeroResources: StoryObj = {
  render: () => {
    const status = {
      ...entraPlugin.status,
      details: {
        imported_users: NaN,
        imported_groups: 0,
      },
    };
    return render(entraPlugin.spec, status);
  },
};

export const ImportFailed: StoryObj = {
  render: () => {
    const status = {
      ...entraPluginErrorStatus,
      details: {
        imported_users: 100,
        imported_groups: 0,
      },
    };
    return render(entraPlugin.spec, status);
  },
};

const render = (
  spec: PluginEntraIdSpec,
  status: PluginStatus<PluginEntraIDStatusDetails>
) => {
  const ctx = createTeleportContextE();
  cfg.isPolicyEnabled = true;
  return (
    <ContextProvider ctx={ctx}>
      <DirectorySyncDetails spec={spec} status={status} />
    </ContextProvider>
  );
};
