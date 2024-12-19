import { generatePath } from 'react-router';

import ossCfg, { UrlResourcesParams } from 'teleport/config';

import generateResourcePath from 'teleport/generateResourcePath';

import { AccessRequestFilter, ResourceId } from 'e-teleport/services/workflow';
import { AccessMonitoringRuleFilter } from 'e-teleport/services/accessmonitoringrule/types';

import type { PluginKind } from 'teleport/services/integrations/types';

const cfg = {
  oss: ossCfg,

  routes: {
    accessGraph: {
      dashboard: '/web/accessgraph',
      browse: '/web/accessgraph/browse',
      crownJewels: '/web/accessgraph/crownjewels',
      graphExplorer: '/web/accessgraph/graph',
      sqlEditor: '/web/accessgraph/sql',
      integrations: '/web/accessgraph/integrations',
    },
    accessLists: '/web/accesslists/:accessListId?',
    accessListNew: '/web/accesslists/new',

    accessMonitoring: {
      base: '/web/accessmonitoring',
      queryEditor: '/web/accessmonitoring/query',
      report: '/web/accessmonitoring/report/:name/:days',
    },

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

    // allow SAML IdP handlers
    samlIdPHandler: '/enterprise/saml-idp/*',

    samlIdPLogin: '/web/saml-idp/login',
    ssoConfirm: '/web/sso_confirm',

    // device trust
    deviceTrust: `/web/devices`,

    // billing
    usageSummarySummary: '/web/cluster/:clusterId/usage-summary',
  },

  api: {
    accessListManagementPath: '/v1/enterprise/accesslist/:accessListId?',
    accessListAddMembersPath: '/v1/enterprise/accesslist/:accessListId/members',
    accessListReviewPath: '/v1/enterprise/accesslist/:accessListId/reviews',
    accessListSuggestionsPath:
      '/v1/enterprise/accessrequest/:requestId/suggestions/accesslist',

    accessGraphQueryPath: '/v1/enterprise/accessgraph/query',

    accessRequestPromotePath: '/v1/enterprise/accessrequest/:requestId/promote',
    accessRequestPath: '/v1/enterprise/accessrequest/:requestId?',
    accessRequestFilterPath:
      '/v1/enterprise/accessrequest?user=:user?&limit=:limit?&startKey=:startKey?&search=:search?&sort=:sort?&scope=:scope?',
    resourceRequestRolesPath:
      '/v1/enterprise/resourcerequestroles?resourceIds=:resourceIds?',

    authConnectorsListPath: '/v1/enterprise/authconnectors',
    samlConnectorsPath: '/v1/enterprise/saml/:name?',
    oidcConnectorsPath: '/v1/enterprise/oidc/:name?',

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
    pluginPath: '/v1/enterprise/plugin/:name?',
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
    surveyCompanyPath: '/v1/enterprise/cloud/survey/company',

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

  getAccessManagementListUrl(accessListId?: string) {
    return generatePath(cfg.api.accessListManagementPath, { accessListId });
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

  getAccessRequestUrl(requestId?: string) {
    return generatePath(cfg.api.accessRequestPath, { requestId });
  },

  getAccessRequestPromoteUrl(requestId: string) {
    return generatePath(cfg.api.accessRequestPromotePath, { requestId });
  },

  getAccessRequestFilterUrl(filter: AccessRequestFilter) {
    return generatePath(cfg.api.accessRequestFilterPath, { ...filter });
  },

  getResourceRequestRolesUrl(resourceIds: ResourceId[]) {
    const stringified = JSON.stringify(resourceIds);

    return generatePath(cfg.api.resourceRequestRolesPath, {
      resourceIds: stringified,
    });
  },

  getAuthConnectorsListUrl() {
    return cfg.api.authConnectorsListPath;
  },

  getSamlConnectorsUrl(name?: string) {
    return generatePath(cfg.api.samlConnectorsPath, { name });
  },

  getOidcConnectorsUrl(name?: string) {
    return generatePath(cfg.api.oidcConnectorsPath, { name });
  },

  getRecoveryTokenUrl(tokenId: string) {
    return generatePath(cfg.api.recoveryTokenPath, { tokenId });
  },

  getPluginUrl(name?: string) {
    return generatePath(cfg.api.pluginPath, { name });
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
    return generatePath(cfg.api.accessMonitoringRule.list, {
      clusterId,
      ...filter,
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
      generatePath(path, { ...p }) +
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

  init(json: object) {
    // this will apply server config by merging it with oss cfg
    ossCfg.init({
      isEnterprise: true,
      routes: cfg.routes,
      nonExactRoutes: this.getNonExactRoutes(),
      ...json,
    });
  },
};

export interface UrlAzureOidcConfigureIdp {
  authConnectorName: string;
  accessGraph: boolean;
}

export type EnterpriseConfig = typeof cfg;

export default cfg;
