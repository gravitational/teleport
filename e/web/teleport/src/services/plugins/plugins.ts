import api from 'teleport/services/api';

import cfg from 'e-teleport/config';

import type { Plugin, PluginKind } from 'teleport/services/integrations';

export const pluginsService = {
  fetchAvailableTypes(): Promise<PluginKind[]> {
    return api.get(cfg.api.pluginTypesPath);
  },

  fetchPlugins(): Promise<Plugin[]> {
    return api.get(cfg.getPluginUrl()).then(makePlugins);
  },

  createPlugin(formData: FormData): Promise<Plugin> {
    return api.postFormData(cfg.getPluginUrl(), formData).then(makePlugin);
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
  const { name, details, statusCode, type } = json;
  return {
    resourceType: 'plugin',
    name,
    details,
    spec: {},
    kind: type,
    statusCode,
  };
}
