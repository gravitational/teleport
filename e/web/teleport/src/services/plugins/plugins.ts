import api from 'teleport/services/api';

import { CtaEvent } from 'teleport/services/userEvent';

import auth from 'teleport/services/auth/auth';

import cfg from 'e-teleport/config';

import type { Plugin, PluginKind } from 'teleport/services/integrations';

export const pluginsService = {
  fetchAvailableTypes(): Promise<PluginKind[]> {
    return api.get(cfg.api.pluginTypesPath);
  },

  fetchPlugins(): Promise<Plugin[]> {
    return api.get(cfg.getPluginUrl()).then(makePlugins);
  },

  async createPlugin(formData: FormData): Promise<Plugin> {
    const webauthnResponse = await auth.getWebauthnResponseForAdminAction(
      false
    );
    return api
      .postFormData(cfg.getPluginUrl(), formData, webauthnResponse)
      .then(makePlugin);
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

export function getCTAForPlugin(plugin: PluginKind) {
  switch (plugin) {
    case 'jamf':
      return CtaEvent.CTA_TRUSTED_DEVICES;
    default:
      return CtaEvent.CTA_UNSPECIFIED;
  }
}
