import api from 'teleport/services/api';

import cfg from 'e-teleport/config';

import type { Plugin } from './types';

const pluginNiceNames = {
  slack: 'Slack',
};

export const pluginsService = {
  fetchPlugins(): Promise<Plugin[]> {
    return api.get(cfg.getPluginUrl(null)).then(json => json.map(makePlugin));
  },

  async deletePlugin(name: string): Promise<void> {
    await api.delete(cfg.getPluginUrl(name));
  },
};

function makePlugin(json: any): Plugin {
  json = json || {};
  const { name, details, status, type } = json;
  return {
    name,
    details,
    status,
    type,
    niceType: pluginNiceNames[type] ?? type,
  };
}
