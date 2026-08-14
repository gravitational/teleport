import cfg from 'e-teleport/config';
import api from 'teleport/services/api';
import auth from 'teleport/services/auth/auth';
import {
  Plugin,
  PluginCredentials,
  PluginEntraIDStatusDetails,
  PluginKind,
  PluginKindToSpec,
  PluginKindToStatusDetails,
  PluginStatus,
} from 'teleport/services/integrations';
import {
  DefaultSystemOktaRequesterRoleName,
  PluginStatusOkta,
} from 'teleport/services/integrations/oktaStatusTypes';
import { CtaEvent } from 'teleport/services/userEvent';

import {
  AwsIcAccounts,
  AwsIcGroupsWithAssignment,
  AwsIcPermissionSets,
  PluginConfigOktaApp,
  PluginConfigOktaGroup,
  PluginUpdateRequest,
} from './types';

export const pluginsService = {
  fetchAvailableTypes(): Promise<PluginKind[]> {
    return api.get(cfg.api.pluginTypesPath);
  },

  fetchPlugins(): Promise<Plugin[]> {
    return api.get(cfg.api.plugin.list).then(makePlugins);
  },

  checkPluginRequiresCleanup(
    kind: PluginKind,
    signal?: AbortSignal
  ): Promise<boolean> {
    return api
      .get(cfg.getPluginNeedsCleanupUrl(kind), signal)
      .then(resp => resp?.needsCleanup);
  },

  cleanupPlugin(kind: PluginKind): Promise<void> {
    return api.put(cfg.getPluginCleanupUrl(kind), null);
  },

  /**
   * Create plugins that do not require OAuth.
   */
  async createStaticAuthPlugin<T extends string>(
    formData: FormData
  ): Promise<Plugin<PluginKindToSpec[T], PluginKindToStatusDetails[T]>> {
    const webauthnResponse =
      await auth.getMfaChallengeResponseForAdminAction(true);

    return await api
      .postFormData(cfg.api.plugin.createStaticAuth, formData, webauthnResponse)
      .then(makePlugin);
  },

  /**
   * Replaces browser URL with the fetched redirect URL.
   */
  async redirectForPluginOAuth(formData: FormData) {
    const webauthnResponse =
      await auth.getMfaChallengeResponseForAdminAction(true);

    return api
      .postFormData(cfg.api.plugin.oAuthStart, formData, webauthnResponse)
      .then(resp => {
        window.location.replace(resp.redirectUrl);
      });
  },

  validatePlugin(formData: FormData) {
    return api.postFormData(cfg.getPluginValidateUrl(), formData);
  },

  async deletePlugin(name: string): Promise<void> {
    await api.delete(cfg.getPluginUrl(name, 'delete'));
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

  fetchPlugin<T extends string>(
    name: string,
    abortSignal?: AbortSignal
  ): Promise<Plugin<PluginKindToSpec[T], PluginKindToStatusDetails[T]>> {
    return api.get(cfg.getPluginUrl(name, 'get'), abortSignal).then(makePlugin);
  },

  updatePlugin<T extends string>(
    req: PluginUpdateRequest<T>
  ): Promise<Plugin<PluginKindToSpec[T], PluginKindToStatusDetails[T]>> {
    return api.put(cfg.api.plugin.update, req).then(makePlugin);
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
  const { name, details, statusCode, type, spec, status, credentials } = json;

  let madeStatus: PluginStatus;
  if (status) {
    madeStatus = {
      code: status.code,
      lastRun: new Date(status.lastRun),
      errorMessage: status.errorMessage,
      lastRawError: status.lastRawError ?? '',
    };

    if (status.details) {
      if (type === 'okta' && status.details?.okta) {
        madeStatus.details = makeOktaPluginStatus(status.details.okta);
      }

      if (type === 'entra-id' && status.details?.entra) {
        madeStatus.details = makeEntraIDPluginStatus(status.details.entra);
      }
    }
  }

  let oAuthCreds: PluginCredentials['OAuthCredentials'] | undefined = undefined;
  if (credentials?.oauth_creds) {
    oAuthCreds = {
      clientId: credentials.oauth_creds.client_id,
      clientSecret: credentials.oauth_creds.client_secret,
    };
  }

  return {
    resourceType: 'plugin',
    name,
    details,
    spec,
    kind: type,
    statusCode,
    status: madeStatus,
    credentials: oAuthCreds ? { OAuthCredentials: oAuthCreds } : undefined,
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
    const { enabled, app_id, app_name, okta_group_everyone_mapped_roles } =
      sso_details;
    status.ssoDetails = {
      enabled: enabled,
      appId: app_id || '',
      appName: app_name || '',
      oktaGroupEveryoneMappedRoles: okta_group_everyone_mapped_roles || [
        DefaultSystemOktaRequesterRoleName,
      ],
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

function makeEntraIDPluginStatus(
  rawEntraIDStatusDetails: any
): PluginEntraIDStatusDetails {
  const { imported_users, imported_groups, sync_mode } =
    rawEntraIDStatusDetails;
  return {
    imported_users: imported_users ?? 0,
    imported_groups: imported_groups ?? 0,
    sync_mode,
  };
}

export function getCTAForPlugin(plugin: PluginKind) {
  switch (plugin) {
    case 'jamf':
    case 'intune':
      return CtaEvent.CTA_TRUSTED_DEVICES;
    case 'entra-id':
      return CtaEvent.CTA_ENTRA_ID;
    default:
      return CtaEvent.CTA_UNSPECIFIED;
  }
}
