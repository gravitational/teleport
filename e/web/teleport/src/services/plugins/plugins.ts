import api from 'teleport/services/api';
import { CtaEvent } from 'teleport/services/userEvent';
import auth from 'teleport/services/auth/auth';

import cfg from 'e-teleport/config';

import { PluginConfigOktaGroup, PluginConfigOktaApp } from './types';

import type { Plugin, PluginKind } from 'teleport/services/integrations';

export const pluginsService = {
  fetchAvailableTypes(): Promise<PluginKind[]> {
    return api.get(cfg.api.pluginTypesPath);
  },

  fetchPlugins(): Promise<Plugin[]> {
    return api.get(cfg.getPluginUrl()).then(makePlugins);
  },

  checkPluginRequiresCleanup(kind: PluginKind): Promise<boolean> {
    return api
      .get(cfg.getPluginNeedsCleanupUrl(kind))
      .then(resp => resp?.needsCleanup);
  },

  cleanupPlugin(kind: PluginKind): Promise<void> {
    return api.put(cfg.getPluginCleanupUrl(kind), null);
  },

  async createPlugin(formData: FormData): Promise<Plugin> {
    const webauthnResponse =
      await auth.getWebauthnResponseForAdminAction(false);
    return api
      .postFormData(cfg.getPluginUrl(), formData, webauthnResponse)
      .then(makePlugin);
  },

  validatePlugin(formData: FormData) {
    return api.postFormData(cfg.getPluginValidateUrl(), formData);
  },

  async deletePlugin(name: string): Promise<void> {
    await api.delete(cfg.getPluginUrl(name));
  },

  getPluginConfigOktaGroups(
    formData: FormData
  ): Promise<PluginConfigOktaGroup[]> {
    return api
      .postFormData(cfg.api.okta.groups, formData)
      .then(resp => resp || []);
  },

  getPluginConfigOktaApps(formData: FormData): Promise<PluginConfigOktaApp[]> {
    return api
      .postFormData(cfg.api.okta.apps, formData)
      .then(resp => resp || []);
  },
};

export function makePlugins(json: any): Plugin[] {
  json = json || [];
  return json.map(makePlugin);
}

function makePlugin(json: any): Plugin {
  json = json || {};
  const { name, details, statusCode, type, spec } = json;
  return {
    resourceType: 'plugin',
    name,
    details,
    spec,
    kind: type,
    statusCode,
  };
}

export function getCTAForPlugin(plugin: PluginKind) {
  switch (plugin) {
    case 'jamf':
      return CtaEvent.CTA_TRUSTED_DEVICES;
    case 'entra-id':
      return CtaEvent.CTA_ENTRA_ID;
    default:
      return CtaEvent.CTA_UNSPECIFIED;
  }
}
