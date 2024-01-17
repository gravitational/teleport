import { generatePath } from 'react-router';

import ossCfg, { UrlResourcesParams } from 'teleport/config';

import generateResourcePath from 'teleport/generateResourcePath';

import { AccessRequestFilter, ResourceId } from 'e-teleport/services/workflow';

const cfg = {
  oss: ossCfg,

  routes: {
    accessGraph: '/web/accessgraph',
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

    // device trust
    deviceTrust: `/web/devices`,

    // billing
    billingSummary: '/web/cluster/:clusterId/billing-summary',
    eubpBillingSummary: '/web/cluster/:clusterId/eubp-billing-summary',
    paymentsInvoices: '/web/cluster/:clusterId/payments-invoices',
    invoiceSettings: '/web/cluster/:clusterId/invoice-settings',
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
    accessRequestFilterPath: '/v1/enterprise/accessrequest?user=:user?',
    resourceRequestRolesPath:
      '/v1/enterprise/resourcerequestroles?resourceIds=:resourceIds?',

    authConnectorsListPath: '/v1/enterprise/authconnectors',
    samlConnectorsPath: '/v1/enterprise/saml/:name?',
    oidcConnectorsPath: '/v1/enterprise/oidc/:name?',
    addressPath: '/v1/enterprise/cloud/address',

    billingPath: '/v1/enterprise/cloud/billing',
    billingSummaryPath: '/v1/enterprise/cloud/billing-summary',
    nonBillableUsageSummaryPath: '/v1/enterprise/cloud/nonbillable-summary',
    cardPath: '/v1/enterprise/cloud/card',
    emailPath: '/v1/enterprise/cloud/billing-email',
    invoiceSettingsPath: '/v1/enterprise/cloud/invoice-settings',
    paymentsInvoicesPath: '/v1/enterprise/cloud/payments-invoices',
    poPath: '/v1/enterprise/cloud/billing-po',
    setupIntentPath: '/v1/enterprise/cloud/setupintent',
    teleportInvitePath: '/v1/enterprise/cloud/teleportinvite',
    teleportCredentialResetPath: '/v1/enterprise/cloud/teleportcredentialreset',

    recoveryStartPath: '/v1/enterprise/cloud/recovery/start',
    recoveryVerifyUserPath: '/v1/enterprise/cloud/recovery/verify',
    recoveryNewCredentialsPath: '/v1/enterprise/cloud/recovery/newcredentials',
    recoveryTokenPath: '/v1/enterprise/cloud/recovery/token/:tokenId',
    recoveryCodesPath: '/v1/enterprise/cloud/recovery/codes',

    upgradeWindowStartPath: '/v1/enterprise/cloud/upgradewindowstart',

    releases: '/v1/enterprise/releases',
    license: '/v1/enterprise/license',

    pluginTypesPath: '/v1/enterprise/plugins/types',
    pluginPath: '/v1/enterprise/plugin/:name?',

    samlIdpPath: '/v1/enterprise/samlidp',

    // TODO(sshah): limit, startKey and search is supported by this API but currently
    // only limit and startKey based pagination is implemented in the UI.
    devices: '/v1/enterprise/devices?limit=:limit?&startKey=:startKey?',

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
  },

  getTrustedDevicesUrl(params: UrlResourcesParams) {
    return generateResourcePath(cfg.api.devices, { ...params });
  },

  getBillingSummaryRoute(clusterId: string) {
    return generatePath(cfg.routes.billingSummary, { clusterId });
  },

  getEubpBillingSummaryRoute(clusterId: string) {
    return generatePath(cfg.routes.eubpBillingSummary, { clusterId });
  },

  getPaymentsInvoicesRoute(clusterId: string) {
    return generatePath(cfg.routes.paymentsInvoices, { clusterId });
  },

  getInvoiceSettingsRoute(clusterId: string) {
    return generatePath(cfg.routes.invoiceSettings, { clusterId });
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

  init(json: object) {
    // this will apply server config by merging it with oss cfg
    ossCfg.init({ isEnterprise: true, routes: cfg.routes, ...json });
  },
};

export default cfg;
