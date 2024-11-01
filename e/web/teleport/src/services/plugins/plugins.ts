import api from 'teleport/services/api';
import { CtaEvent } from 'teleport/services/userEvent';
import auth from 'teleport/services/auth/auth';
import { PluginStatusOkta } from 'teleport/services/integrations/oktaStatusTypes';

import cfg from 'e-teleport/config';

import {
  PluginConfigOktaGroup,
  PluginConfigOktaApp,
  AwsIcAccounts,
  AwsIcGroupsWithAssignment,
  AwsIcPermissionSets,
} from './types';

import type {
  Plugin,
  PluginKind,
  PluginStatus,
} from 'teleport/services/integrations';

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

  fetchPlugin(name: string): Promise<Plugin> {
    return api.get(cfg.getPluginUrl(name)).then(makePlugin);
  },

  getAwsIcAccounts(
    params: fetchAwsIcResourceRequest
  ): Promise<AwsIcAccounts[]> {
    return api.post(cfg.getAwsIcPluginPreviewAccountWithPermSetsUrl(), params);
  },

  getAwsIcGroupsWithPermissionAssignments(
    params: fetchAwsIcResourceRequest
  ): Promise<AwsIcGroupsWithAssignment[]> {
    return api.post(cfg.getAwsIcPluginPreviewGroupsWithAssignmentUrl(), params);
  },

  getAwsIcPermissionSets(
    params: fetchAwsIcResourceRequest
  ): Promise<AwsIcPermissionSets[]> {
    return api.post(cfg.getAwsIcPluginPreviewPermissionSetsUrl(), params);
  },
};

type fetchAwsIcResourceRequest = {
  integrationName: string;
  arn: string;
  region: string;
};

export function makePlugins(json: any): Plugin[] {
  json = json || [];
  return json.map(makePlugin);
}

function makePlugin(json: any): Plugin {
  json = json || {};
  const { name, details, statusCode, type, spec, status } = json;

  let madeStatus: PluginStatus;
  if (status) {
    madeStatus = {
      code: status.code,
      lastRun: new Date(status.lastRun),
      errorMessage: status.errorMessage,
    };

    if (status.details) {
      if (type === 'okta' && status.details?.okta) {
        madeStatus.details = makeOktaPluginStatus(status.details.okta);
      }
    }
  }

  return {
    resourceType: 'plugin',
    name,
    details,
    spec,
    kind: type,
    statusCode,
    status: madeStatus,
  };
}

function makeOktaPluginStatus(rawOktaDetails): PluginStatusOkta {
  const {
    sso_details,
    app_group_sync_details,
    users_sync_details,
    scim_details,
    access_lists_sync_details,
  } = rawOktaDetails;

  let status: PluginStatusOkta = {};

  if (sso_details) {
    const { enabled, app_id, app_name } = sso_details;
    status.ssoDetails = {
      enabled: enabled,
      appId: app_id || '',
      appName: app_name || '',
    };
  }

  if (app_group_sync_details) {
    const {
      enabled,
      status_code,
      last_successful,
      last_failed,
      num_apps_synced,
      num_groups_synced,
      error,
    } = app_group_sync_details;
    status.appGroupSyncDetails = {
      enabled,
      statusCode: status_code,
      lastSuccess: new Date(last_successful),
      lastFailed: new Date(last_failed),
      numApps: num_apps_synced || 0,
      numGroups: num_groups_synced || 0,
      error,
    };
  }

  if (users_sync_details) {
    const {
      enabled,
      status_code,
      last_successful,
      last_failed,
      num_users_synced,
      error,
    } = users_sync_details;

    status.usersSyncDetails = {
      enabled,
      statusCode: status_code,
      lastSuccess: new Date(last_successful),
      lastFailed: new Date(last_failed),
      numUsers: num_users_synced || 0,
      error,
    };
  }

  if (scim_details) {
    status.scimDetails = { enabled: scim_details.enabled };
  }

  if (access_lists_sync_details) {
    const {
      enabled,
      status_code,
      last_successful,
      last_failed,
      app_filters,
      num_apps_synced,
      group_filters,
      num_groups_synced,
      error,
    } = access_lists_sync_details;
    status.accessListsSyncDetails = {
      enabled,
      statusCode: status_code,
      lastFailed: new Date(last_failed),
      lastSuccess: new Date(last_successful),
      appFilters: app_filters || [],
      numApps: num_apps_synced || 0,
      groupFilters: group_filters || [],
      numGroups: num_groups_synced || 0,
      error,
    };
  }

  return status;
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
