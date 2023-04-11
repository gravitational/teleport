import api from 'teleport/services/api';

import cfg from 'e-teleport/config';

import type { Plugin } from 'teleport/services/integrations';

export const pluginsService = {
  fetchAvailableTypes(): Promise<string[]> {
    return api.get(cfg.api.pluginTypesPath);
  },

  fetchPlugins(): Promise<Plugin[]> {
    return api.get(cfg.getPluginUrl(null)).then(makePlugins);
  },

  async deletePlugin(name: string): Promise<void> {
    await api.delete(cfg.getPluginUrl(name));
  },
};

export function makePlugins(json: any): Plugin[] {
  json = json || [];
  return json.map(makePlugin);
}

function makePlugin(json: any): Plugin {
  json = json || {};
  const { name, details, status, type } = json;
  return {
    resourceType: 'plugin',
    name,
    details,
    spec: {
      statusDescription: status.description,
    },
    kind: type,
    statusCode: status.code,
  };
}
