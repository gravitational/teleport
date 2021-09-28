import { generatePath } from 'react-router';
import ossCfg from 'teleport/config';
import { AccessRequestFilter } from 'e-teleport/services/workflow';

const cfg = {
  oss: ossCfg,

  routes: {
    requests: '/web/requests/:requestId?',
    requestNew: '/web/requests/new',

    billing: '/web/billing',
    billingUsage: '/web/billing/usage',
    billingAccount: '/web/billing/account',
    billingInvoices: '/web/billing/invoices',

    recovery: '/web/recovery/',
    recoveryForgotPassword: '/web/recovery/forgot/password',
    recoveryForgotDevice: '/web/recovery/forgot/device',
    recoverySteps: '/web/recovery/steps/:tokenId',
    recoveryStepVerify: '/web/recovery/steps/:tokenId/verify',
    recoveryStepNewPassword: '/web/recovery/steps/:tokenId/new/password',
    recoveryStepNewDevice: '/web/recovery/steps/:tokenId/new/device',
    recoveryStepDevices: '/web/recovery/steps/:tokenId/devices',
    recoveryStepCodes: '/web/recovery/steps/:tokenId/codes',
  },

  api: {
    accessRequestPath: '/v1/enterprise/accessrequest/:requestId?',
    accessRequestFilterPath: '/v1/enterprise/accessrequest?user=:user?',
    authConnectorsListPath: '/v1/enterprise/authconnectors',
    samlConnectorsPath: '/v1/enterprise/saml/:name?',
    oidcConnectorsPath: '/v1/enterprise/oidc/:name?',
    billingPath: '/v1/enterprise/cloud/billing',
    cyclesPath: '/v1/enterprise/cloud/cycles',
    invoicesPath: '/v1/enterprise/cloud/invoices',
    cardPath: '/v1/enterprise/cloud/card',
    accountPath: '/v1/enterprise/cloud/account',

    recoveryStartPath: '/v1/enterprise/cloud/recovery/start',
    recoveryVerifyUserPath: '/v1/enterprise/cloud/recovery/verify',
    recoveryNewCredentialsPath: '/v1/enterprise/cloud/recovery/newcredentials',
    recoveryTokenPath: '/v1/enterprise/cloud/recovery/token/:tokenId',
    recoveryNewCodesPath: '/v1/enterprise/cloud/recovery/codes',
  },

  getAccessRequestRoute(requestId?: string) {
    return generatePath(cfg.routes.requests, { requestId });
  },

  getAccessRequestUrl(requestId?: string) {
    return generatePath(cfg.api.accessRequestPath, { requestId });
  },

  getAccessRequestFilterUrl(filter: AccessRequestFilter) {
    return generatePath(cfg.api.accessRequestFilterPath, { ...filter });
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

  init(json: object) {
    // this will apply server config by merging it with oss cfg
    ossCfg.init({ isEnterprise: true, routes: cfg.routes, ...json });
  },
};

export default cfg;
