import { generatePath } from 'react-router';

import { SortType } from 'design/DataTable/types';
import { ResourceId } from 'shared/services/accessRequests';
import { mergeDeep } from 'shared/utils/highbar';

import { AccessMonitoringRuleFilter } from 'e-teleport/services/accessmonitoringrule/types';
import { AccessRequestFilter } from 'e-teleport/services/workflow';
import ossCfg, { UrlResourcesParams } from 'teleport/config';
import generateResourcePath from 'teleport/generateResourcePath';
import type { PluginKind } from 'teleport/services/integrations/types';

/**
 * Generates a URL path by substituting named parameters.
 * Unlike react-router's generatePath, this function also supports
 * parameters in query strings (e.g., `?param=:value`).
 */
function generateFullPath(
  pattern: string,
  params: Record<string, string | number | boolean | undefined>
): string {
  const queryStart = pattern.indexOf('?');
  const hasQuery = queryStart !== -1;
  const pathPart = hasQuery ? pattern.slice(0, queryStart) : pattern;
  const queryPart = hasQuery ? pattern.slice(queryStart) : '';

  const pathParams = Object.fromEntries(
    Object.entries(params).map(([key, value]) => [
      key,
      value === undefined || value === '' ? null : String(value),
    ])
  ) as Record<string, string | null>;
  const processedPath = generatePath(pathPart, pathParams);

  const processedQuery = queryPart.replace(
    /:([A-Za-z_][A-Za-z0-9_]*)(\?)?/g,
    (fullMatch, key: string, optional: string | undefined) => {
      const value = params[key];
      if (value === undefined || value === '') {
        return optional ? '' : fullMatch;
      }
      return encodeURIComponent(String(value));
    }
  );

  return processedPath + processedQuery;
}

export const enterpriseRoutes = {
  accessGraph: {
    dashboard: '/web/accessgraph',
    browse: '/web/accessgraph/browse',
    alerts: '/web/accessgraph/alerts',
    investigate: '/web/accessgraph/investigate',
    crownJewels: '/web/accessgraph/crownjewels',
    graphExplorer: '/web/accessgraph/graph',
    sqlEditor: '/web/accessgraph/sql',
    integrations: '/web/accessgraph/integrations',
  },
  accessLists: '/web/accesslists/:accessListId?',
  accessListsList: '/web/accesslists',
  accessListNew: '/web/accesslists/new',

  accessMonitoring: {
    base: '/web/accessmonitoring',
    queryEditor: '/web/accessmonitoring/query',
    report: '/web/accessmonitoring/report/:name/:days',
  },

  accessAutomations: `/web/accessautomations`,
  accessAutomationNew: `/web/accessautomations/new`,

  requests: '/web/requests/:requestId?',
  requestNew: '/web/cluster/:clusterId/requests/new',

  accountRecovery: '/web/account/recovery',

  recovery: '/web/recovery/',
  recoveryForgotPassword: '/web/recovery/forgot/password',
  recoveryForgotDevice: '/web/recovery/forgot/device',
  recoverySteps: '/web/recovery/steps/:tokenId',
  recoveryStepVerify: '/web/recovery/steps/:tokenId/verify',
  recoveryStepNewPassword: '/web/recovery/steps/:tokenId/new/password',
  recoveryStepNewDevice: '/web/recovery/steps/:tokenId/new/device',
  recoveryStepDevices: '/web/recovery/steps/:tokenId/devices',
  recoveryStepCodes: '/web/recovery/steps/:tokenId/codes',

  /**
   * ssoNewConnectorList is a page which lists possible auth connector types to add, similar to Discover.
   */
  ssoNewConnectorList: '/web/sso/new',

  // allow SAML IdP handlers
  samlIdPHandler: '/enterprise/saml-idp/*',

  samlIdPLogin: '/web/saml-idp/login',
  ssoConfirm: '/web/sso_confirm',

  // device trust
  deviceTrust: `/web/devices`,

  // billing
  usageSummarySummary: '/web/cluster/:clusterId/usage-summary',

  sessionSummariesManagement: '/web/cluster/:clusterId/recordings/summaries',
};

const cfg = {
  oss: ossCfg,

  routes: { ...enterpriseRoutes },

  api: {
    // TODO(kimlisa): move accessListXXX to the "accessList" object.
    accessListManagementPath: '/v1/enterprise/accesslist/:accessListId?',
    accessListManagementPathV2:
      '/v2/enterprise/accesslists?limit=:limit?&startKey=:startKey?&search=:search?&sort=:sort?&owners=:owners?&origin=:origin?',
    accessListAddMembersPath: '/v1/enterprise/accesslist/:accessListId/members',
    accessListReviewPath: '/v1/enterprise/accesslist/:accessListId/reviews',
    accessListSuggestionsPath:
      '/v1/enterprise/accessrequest/:requestId/suggestions/accesslist',
    userAccessListsPath: '/v1/enterprise/users/:username/accesslists',

    accessList: {
      reviews:
        '/v1/enterprise/accesslist/:accessListId/reviews?limit=:limit?&startKey=:startKey?',
    },

    accessListPreset: {
      create: '/v1/enterprise/accesslistpreset',
      update: '/v1/enterprise/accesslistpreset/:accessListId',
    },

    accessGraphSettingsPath: '/v1/enterprise/accessgraphsettings',
    accessGraphQueryPath: '/v1/enterprise/accessgraph/query',
    accessGraphRoleTesterPath:
      '/v1/enterprise/accessgraph/graph/tester/teleport/role/v1',

    accessRequestPromotePath: '/v1/enterprise/accessrequest/:requestId/promote',
    accessRequestPath: '/v1/enterprise/accessrequest/:requestId?',
    accessRequestFilterPath:
      '/v1/enterprise/accessrequest?user=:user?&limit=:limit?&startKey=:startKey?&search=:search?&sort=:sort?&scope=:scope?',
    resourceRequestRolesPath:
      '/v1/enterprise/resourcerequestroles?resourceIds=:resourceIds?',

    authConnectorsListPath: '/v1/enterprise/authconnectors',
    samlConnectorsPath: '/v1/enterprise/saml/:name?',
    samlConnectorSpecificPath: '/v1/enterprise/saml/connector/:name',
    oidcConnectorsPath: '/v1/enterprise/oidc/:name?',
    oidcConnectorSpecificPath: '/v1/enterprise/oidc/connector/:name',

    billingSummaryPath: '/v1/enterprise/cloud/billing-summary',
    nonBillableUsageSummaryPath: '/v1/enterprise/cloud/nonbillable-summary',
    teleportInvitePath: '/v1/enterprise/cloud/teleportinvite',
    teleportCredentialResetPath: '/v1/enterprise/cloud/teleportcredentialreset',

    recoveryStartPath: '/v1/enterprise/cloud/recovery/start',
    recoveryVerifyUserPath: '/v1/enterprise/cloud/recovery/verify',
    recoveryNewCredentialsPath: '/v1/enterprise/cloud/recovery/newcredentials',
    recoveryTokenPath: '/v1/enterprise/cloud/recovery/token/:tokenId',
    recoveryCodesPath: '/v1/enterprise/cloud/recovery/codes',

    upgradeWindowStartPath:
      '/v1/enterprise/sites/:clusterId/upgradewindowstart',

    releases: '/v1/enterprise/releases',
    license: '/v1/enterprise/license',

    pluginTypesPath: '/v1/enterprise/plugins/types',
    plugin: {
      list: '/v1/enterprise/plugin',
      get: '/v1/enterprise/plugin/:name',
      delete: '/v1/enterprise/plugin/:name',
      update: '/v1/enterprise/plugin',
      createStaticAuth: '/v1/enterprise/plugins/staticauth',
      oAuthStart: '/v1/enterprise/plugins/oauth/start',
    },
    pluginValidatePath: '/v1/enterprise/plugins/validate',
    pluginNeedsCleanupPath: '/v1/enterprise/plugins/needscleanup/:kind',
    pluginCleanupPath: '/v1/enterprise/plugins/cleanup/:kind',
    pluginStatusPath: '/v1/enterprise/plugins/status/:name',

    okta: {
      groups: '/v1/enterprise/pluginconfig/okta/groups',
      apps: '/v1/enterprise/pluginconfig/okta/apps',
    },

    samlIdpPath: '/v1/enterprise/samlidp',
    // samlIdPMetadataValuesPath is served by SAML IdP, returns metadata values.
    samlIdPMetadataValuesPath: '/enterprise/saml-idp/metadata-values',
    // samlIdPMetadataFilePath is served by SAML IdP, returns metadata XML file.
    samlIdPMetadataFilePath: '/enterprise/saml-idp/metadata',

    // TODO(sshah): limit, startKey and search is supported by this API but currently
    // only limit and startKey based pagination is implemented in the UI.
    devices: '/v1/enterprise/devices?limit=:limit?&startKey=:startKey?',
    deviceByUser:
      '/v1/enterprise/user/devices?limit=:limit?&startKey=:startKey?',

    surveyPath: '/v1/enterprise/cloud/survey',

    accessMonitoring: {
      schema: '/v1/webapi/sites/:clusterId/audit/schema',
      reports: '/v1/webapi/sites/:clusterId/audit/reports',
      reportRun: '/v1/webapi/sites/:clusterId/audit/reports/:name/run',
      reportResult:
        '/v1/webapi/sites/:clusterId/audit/reports/:name/result/days/:timeframe',
      reportState:
        '/v1/webapi/sites/:clusterId/audit/reports/:name/state/days/:timeframe',
      queryRun: '/v1/webapi/sites/:clusterId/audit/queries/run',
      result: '/v1/webapi/sites/:clusterId/audit/queries/result',
    },

    accessMonitoringRule: {
      list: '/v1/webapi/sites/:clusterId/accessmonitoringrule?limit=:limit?&startKey=:startKey?&subject=:subject?',
      create: '/v1/webapi/sites/:clusterId/accessmonitoringrule',
      update: '/v1/webapi/sites/:clusterId/accessmonitoringrule/:name',
      delete: '/v1/webapi/sites/:clusterId/accessmonitoringrule/:name',
      terraform:
        '/v1/webapi/sites/:clusterId/accessmonitoringrule/:name/terraform',
    },

    externalAuditStorage: {
      generate:
        '/v1/webapi/sites/:clusterId/integration/externalauditstorage/generate',
      bootstrap:
        '/v1/webapi/scripts/integration/externalauditstorage-bootstrap.sh',
      promote:
        '/v1/webapi/sites/:clusterId/integration/externalauditstorage/promote',
      cluster:
        '/v1/webapi/sites/:clusterId/integration/externalauditstorage/cluster',
      draft:
        '/v1/webapi/sites/:clusterId/integration/externalauditstorage/draft',
      testConnection:
        '/v1/webapi/sites/:clusterId/integration/externalauditstorage/test',
    },

    azureOidcConfigureScriptPath:
      '/webapi/scripts/integrations/configure/azureoidc.sh?authConnectorName=:authConnectorName',

    awsIdentityCenter: {
      previewAccountWithPermSets:
        '/v1/enterprise/pluginconfig/aws-ic/preview/accounts-with-permission-sets',
      previewGroupWithAssignment:
        '/v1/enterprise/pluginconfig/aws-ic/preview/groups-with-assignments',
      previewPermissionSets:
        '/v1/enterprise/pluginconfig/aws-ic/preview/permission-sets',
    },

    contacts: '/v1/enterprise/sites/:clusterId/contact',
    clientIpRestrictions:
      '/v1/enterprise/sites/:clusterId/clientiprestrictions',

    sessionRecordingSummary:
      '/v1/webapi/sites/:clusterId/session-summaries/:sessionId',

    inference: {
      testModel: '/v1/webapi/sites/:clusterId/inference/test-model',
      policies: '/v1/webapi/sites/:clusterId/inference/policies',
      policy: '/v1/webapi/sites/:clusterId/inference/policies/:name',
      secrets: '/v1/webapi/sites/:clusterId/inference/secrets',
      secret: '/v1/webapi/sites/:clusterId/inference/secrets/:name',
      models: '/v1/webapi/sites/:clusterId/inference/models',
      model: '/v1/webapi/sites/:clusterId/inference/models/:name',
    },
  },

  getNonExactRoutes() {
    // These routes will not be exact matched when deciding if it is a valid route
    // to redirect to when a user is unauthenticated.
    // This is useful for routes that can be infinitely nested, e.g. `
    // /web/accessgraph` and `/web/accessgraph/integrations/new`
    // (`/web/accessgraph/*` wouldn't work as it doesn't match `/web/accessgraph`)

    return [this.routes.accessGraph.dashboard];
  },

  getWindowUpgradeStartUrl(clusterId: string) {
    return generatePath(cfg.api.upgradeWindowStartPath, { clusterId });
  },

  getTrustedDevicesUrl(params: UrlResourcesParams) {
    return generateResourcePath(cfg.api.devices, { ...params });
  },

  getTrustedDevicesByUserUrl(params: UrlResourcesParams) {
    return generateResourcePath(cfg.api.deviceByUser, { ...params });
  },

  getUsageSummarySummaryRoute(clusterId: string) {
    return generatePath(cfg.routes.usageSummarySummary, { clusterId });
  },

  getAccessListManagementRoute(accessListId?: string) {
    return generatePath(cfg.routes.accessLists, { accessListId });
  },

  getAccessRequestRoute(requestId?: string) {
    return generatePath(cfg.routes.requests, { requestId });
  },

  getNewAccessRequestRoute(clusterId: string) {
    return generatePath(cfg.routes.requestNew, { clusterId });
  },

  getAccessAutomationRoute() {
    return generatePath(cfg.routes.accessAutomations);
  },

  getNewAccessAutomationRoute() {
    return generatePath(cfg.routes.accessAutomationNew);
  },

  getAccessManagementListUrl(accessListId?: string) {
    return generatePath(cfg.api.accessListManagementPath, { accessListId });
  },

  getAccessListWithPresetUrl(
    req: { action: 'create' } | { action: 'update'; accessListId: string }
  ) {
    const action = req.action;
    switch (action) {
      case 'create':
        return generatePath(cfg.api.accessListPreset.create);
      case 'update':
        return generatePath(cfg.api.accessListPreset.update, {
          accessListId: req.accessListId,
        });
      default:
        action satisfies never;
        return '';
    }
  },

  getAccessManagementListUrlV2(params: {
    sort?: SortType;
    search?: string;
    limit?: number;
    startKey?: string;
    owners?: string[];
    origin?: string;
  }) {
    return generateResourcePath(cfg.api.accessListManagementPathV2, {
      sort: params.sort,
      startKey: params.startKey || undefined,
      limit: params.limit,
      search: params.search || undefined,
      owners: params.owners || [],
      origin: params.origin || undefined,
    });
  },

  getAccessListMembersUrl(accessListId: string) {
    return generatePath(cfg.api.accessListAddMembersPath, { accessListId });
  },

  getAccessListReviewUrl(accessListId: string) {
    return generatePath(cfg.api.accessListReviewPath, { accessListId });
  },

  getAccessListSuggestionsUrl(requestId: string) {
    return generatePath(cfg.api.accessListSuggestionsPath, { requestId });
  },

  getUserAccessListsUrl(username: string) {
    return generatePath(cfg.api.userAccessListsPath, { username });
  },

  getAccessListUrl(req: {
    action: 'reviews';
    params: { accessListId: string; limit?: number; startKey?: string };
  }) {
    const action = req.action;
    switch (action) {
      case 'reviews':
        return generateFullPath(cfg.api.accessList.reviews, {
          accessListId: req.params.accessListId,
          limit: req.params.limit || undefined,
          startKey: req.params.startKey || undefined,
        });
      default:
        action satisfies never;
    }
  },

  getAccessRequestUrl(requestId?: string) {
    return generatePath(cfg.api.accessRequestPath, { requestId });
  },

  getAccessRequestPromoteUrl(requestId: string) {
    return generatePath(cfg.api.accessRequestPromotePath, { requestId });
  },

  getAccessRequestFilterUrl(filter: AccessRequestFilter) {
    return generateFullPath(cfg.api.accessRequestFilterPath, { ...filter });
  },

  getResourceRequestRolesUrl(resourceIds: ResourceId[]) {
    const stringified = JSON.stringify(resourceIds);

    return generateFullPath(cfg.api.resourceRequestRolesPath, {
      resourceIds: stringified,
    });
  },

  getAuthConnectorsListUrl() {
    return cfg.api.authConnectorsListPath;
  },

  getSamlConnectorsUrl(name?: string) {
    return generatePath(cfg.api.samlConnectorsPath, { name });
  },

  getSamlConnectorSpecificUrl(name: string) {
    return generatePath(cfg.api.samlConnectorSpecificPath, { name });
  },

  getOidcConnectorsUrl(name?: string) {
    return generatePath(cfg.api.oidcConnectorsPath, { name });
  },

  getOidcConnectorSpecificUrl(name: string) {
    return generatePath(cfg.api.oidcConnectorSpecificPath, { name });
  },

  getRecoveryTokenUrl(tokenId: string) {
    return generatePath(cfg.api.recoveryTokenPath, { tokenId });
  },

  getPluginUrl(name: string, action: 'get' | 'delete') {
    switch (action) {
      case 'delete':
        return generatePath(cfg.api.plugin.delete, { name });
      case 'get':
        return generatePath(cfg.api.plugin.get, { name });
      default:
        action satisfies never;
    }
  },

  getPluginNeedsCleanupUrl(kind: PluginKind) {
    return generatePath(cfg.api.pluginNeedsCleanupPath, { kind });
  },

  getPluginCleanupUrl(kind: PluginKind) {
    return generatePath(cfg.api.pluginCleanupPath, { kind });
  },

  getPluginValidateUrl() {
    return generatePath(cfg.api.pluginValidatePath);
  },

  getPluginStatusUrl(name: string) {
    return generatePath(cfg.api.pluginStatusPath, { name });
  },

  getAccessMonitoringReportRoute(name: string, days: number) {
    return generatePath(cfg.routes.accessMonitoring.report, { name, days });
  },

  getAccessMonitoringReportsUrl(clusterId: string) {
    return generatePath(cfg.api.accessMonitoring.reports, { clusterId });
  },

  getAccessMonitoringSchemaUrl(clusterId: string) {
    return generatePath(cfg.api.accessMonitoring.schema, { clusterId });
  },

  getAccessMonitoringReportUrl(
    clusterId: string,
    reportName: string,
    timeframe: number
  ) {
    return generatePath(cfg.api.accessMonitoring.reportResult, {
      clusterId,
      name: reportName,
      timeframe,
    });
  },

  getAccessMonitoringReportRunUrl(clusterId: string, reportName: string) {
    return generatePath(cfg.api.accessMonitoring.reportRun, {
      clusterId,
      name: reportName,
    });
  },

  getAccessMonitoringQueryRunUrl(clusterId: string) {
    return generatePath(cfg.api.accessMonitoring.queryRun, { clusterId });
  },

  getAccessMonitoringQueryResultUrl(clusterId: string) {
    return generatePath(cfg.api.accessMonitoring.result, {
      clusterId,
    });
  },

  getAccessMonitoringReportStateUrl(
    clusterId: string,
    reportName: string,
    timeframe: number
  ) {
    return generatePath(cfg.api.accessMonitoring.reportState, {
      clusterId,
      name: reportName,
      timeframe,
    });
  },

  getAccessMonitoringRulesUrl(
    clusterId: string,
    filter: AccessMonitoringRuleFilter
  ) {
    return generateFullPath(cfg.api.accessMonitoringRule.list, {
      clusterId,
      ...filter,
      startKey: filter?.startKey || undefined,
    });
  },

  getAccessMonitoringRuleUpdateUrl(clusterId: string, name: string) {
    return generatePath(cfg.api.accessMonitoringRule.update, {
      clusterId,
      name,
    });
  },

  getAccessMonitoringRuleDeleteUrl(clusterId: string, name: string) {
    return generatePath(cfg.api.accessMonitoringRule.delete, {
      clusterId,
      name,
    });
  },

  getAccessMonitoringRuleCreateUrl(clusterId: string) {
    return generatePath(cfg.api.accessMonitoringRule.create, {
      clusterId,
    });
  },

  getAccessMonitoringRuleTerraformUrl(clusterId: string, name: string) {
    return generatePath(cfg.api.accessMonitoringRule.terraform, {
      clusterId,
      name,
    });
  },

  getExternalAuditStorageGenerateUrl(clusterId: string) {
    return generatePath(cfg.api.externalAuditStorage.generate, {
      clusterId,
    });
  },

  getExternalAuditStoragePromoteUrl(clusterId: string) {
    return generatePath(cfg.api.externalAuditStorage.promote, {
      clusterId,
    });
  },

  getExternalAuditStorageClusterUrl(clusterId: string) {
    return generatePath(cfg.api.externalAuditStorage.cluster, {
      clusterId,
    });
  },

  getExternalAuditStorageDraftUrl(clusterId: string) {
    return generatePath(cfg.api.externalAuditStorage.draft, {
      clusterId,
    });
  },

  getAzureOidcConfigureScriptUrl(p: UrlAzureOidcConfigureIdp) {
    let path = cfg.api.azureOidcConfigureScriptPath;
    return (
      cfg.oss.baseUrl +
      generateFullPath(path, { ...p }) +
      (p.accessGraph ? '&accessGraph=true' : '')
    );
  },

  getAwsIcPluginPreviewAccountWithPermSetsUrl() {
    return generatePath(cfg.api.awsIdentityCenter.previewAccountWithPermSets);
  },

  getAwsIcPluginPreviewGroupsWithAssignmentUrl() {
    return generatePath(cfg.api.awsIdentityCenter.previewGroupWithAssignment);
  },

  getAwsIcPluginPreviewPermissionSetsUrl() {
    return generatePath(cfg.api.awsIdentityCenter.previewPermissionSets);
  },

  getContactsUrl(clusterId: string) {
    return generatePath(cfg.api.contacts, {
      clusterId,
    });
  },

  getClientIpRestrictionsUrl(clusterId: string) {
    return generatePath(cfg.api.clientIpRestrictions, { clusterId });
  },

  getSessionRecordingSummaryUrl(clusterId: string, sessionId: string) {
    return generatePath(cfg.api.sessionRecordingSummary, {
      clusterId,
      sessionId,
    });
  },

  getListInferencePoliciesUrl(
    clusterId: string,
    limit?: number,
    startKey?: string
  ) {
    return (
      generatePath(cfg.api.inference.policies, { clusterId }) +
      createLimitStartKeyParams(limit, startKey)
    );
  },

  getListInferenceModelsUrl(
    clusterId: string,
    limit?: number,
    startKey?: string
  ) {
    return (
      generatePath(cfg.api.inference.models, { clusterId }) +
      createLimitStartKeyParams(limit, startKey)
    );
  },

  getListInferenceSecretsUrl(
    clusterId: string,
    limit?: number,
    startKey?: string
  ) {
    return (
      generatePath(cfg.api.inference.secrets, { clusterId }) +
      createLimitStartKeyParams(limit, startKey)
    );
  },

  getTestInferenceModelUrl(clusterId: string) {
    return generatePath(cfg.api.inference.testModel, { clusterId });
  },

  getInferenceSecretsUrl(clusterId: string) {
    return generatePath(cfg.api.inference.secrets, { clusterId });
  },

  getInferenceSecretUrl(clusterId: string, name: string) {
    return generatePath(cfg.api.inference.secret, { clusterId, name });
  },

  getInferenceModelsUrl(clusterId: string) {
    return generatePath(cfg.api.inference.models, { clusterId });
  },

  getInferenceModelUrl(clusterId: string, name: string) {
    return generatePath(cfg.api.inference.model, { clusterId, name });
  },

  getInferencePoliciesUrl(clusterId: string) {
    return generatePath(cfg.api.inference.policies, { clusterId });
  },

  getInferencePolicyUrl(clusterId: string, name: string) {
    return generatePath(cfg.api.inference.policy, { clusterId, name });
  },

  getSessionSummariesManagementRoute(clusterId: string) {
    return generatePath(cfg.routes.sessionSummariesManagement, { clusterId });
  },

  init(json: object) {
    const {
      routes: serverRoutes,
      nonExactRoutes: serverNonExactRoutes,
      ...rest
    } = (json ?? {}) as {
      routes?: Record<string, unknown>;
      nonExactRoutes?: string[];
      [key: string]: unknown;
    };

    const routes = mergeDeep({}, enterpriseRoutes, serverRoutes ?? {});

    // This applies server config by merging it with OSS config,
    // while preserving enterprise route defaults.
    ossCfg.init({
      ...rest,
      isEnterprise: true,
      routes,
      nonExactRoutes: serverNonExactRoutes ?? this.getNonExactRoutes(),
    });
  },
};

function createLimitStartKeyParams(limit?: number, startKey?: string) {
  const params = new URLSearchParams();

  if (typeof limit !== 'undefined') {
    params.append('limit', limit.toString());
  }

  if (startKey) {
    params.append('startKey', startKey);
  }

  if (params.size === 0) {
    return '';
  }

  return `?${params.toString()}`;
}

export interface UrlAzureOidcConfigureIdp {
  authConnectorName: string;
  accessGraph: boolean;
}

export type EnterpriseConfig = typeof cfg;

export default cfg;
