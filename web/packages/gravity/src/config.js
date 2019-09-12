/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import $ from 'jQuery';
import { at } from 'lodash';
import { generatePath } from 'react-router';
import { Auth2faTypeEnum } from 'gravity/services/enums';

// dummy placeholder for legacy APIs
const accountId = '00000000-0000-0000-0000-000000000001';

const cfg = {

  defaultSiteId: 'undefined',

  logo: null,

  systemInfo: {
    clusterName: ''
  },

  baseUrl: window.location.origin,

  dateTimeFormat: 'DD/MM/YYYY HH:mm:ss',

  dateFormat: 'DD/MM/YYYY',

  isEnterprise: false,

  auth: {
    second_factor: Auth2faTypeEnum.DISABLED,
    oids: [],
    saml: [],
  },

  user: {
    // logo to be displayed on login/forgot password screens
    logo: null,

    login: {
      headerText: 'Gravity'
    },
  },

  agentReport: {
    provision: {
      interfaces: {
        ipv4: {
          labelText: 'IP Address',
          toolipText: 'IP address used to communicate within the cluster'
        }
      },
      mounts: {
        /*
        *  'name': {
        *    labelText: 'input field label',
        *    toolipText: 'input field tooltip'
        * }
        */
      }
    }
  },

  routes: {
    // public routes
    app: '/web',
    login: '/web/login',
    loginFailed: '/web/msg/error/login_failed',
    loginSuccess: '/web/msg/info/login_success',
    logout: '/web/logout',
    userInvite: '/web/newuser/:token',
    userReset: '/web/reset/:token',

    // default app entry point
    defaultEntry: '/web/portal',

    // installer
    installerBase: '/web/installer',
    installerApp: '/web/installer/new/:repository/:name/:version',
    installerCluster: '/web/installer/site/:siteId',
    installerComplete: '/web/installer/site/:siteId/complete/',

    // site
    siteBase: '/web/site/:siteId',
    siteUsers: '/web/site/:siteId/users',
    siteSettings: '/web/site/:siteId/settings',
    siteCertificate: '/web/site/:siteId/certificate',
    siteLicense: '/web/site/:siteId/license',
    siteAudit: '/web/site/:siteId/audit',
    siteOffline: '/web/site/:siteId/offline',
    siteServers: '/web/site/:siteId/servers',
    siteLogs: '/web/site/:siteId/logs',
    siteMonitor: '/web/site/:siteId/monitor',
    siteMonitorPod: '/web/site/:siteId/monitor/dashboard/db/pods?var-namespace=:namespace&var-podname=:podName',
    siteK8s: '/web/site/:siteId/k8s/:namespace?/:category?',
    siteK8sConfigMaps: '/web/site/:siteId/k8s/:namespace/configs',
    siteK8sSecrets: '/web/site/:siteId/k8s/:namespace/secrets',
    siteK8sPods: '/web/site/:siteId/k8s/:namespace/pods',
    siteK8sServices: '/web/site/:siteId/k8s/:namespace/services',
    siteK8sJobs: '/web/site/:siteId/k8s/:namespace/jobs',
    siteK8sDaemonSets: '/web/site/:siteId/k8s/:namespace/daemons',
    siteK8sDeployments: '/web/site/:siteId/k8s/:namespace/deployments',
    console: '/web/site/:siteId/console',
    consoleSession: '/web/site/:siteId/console/session/:sid',
    consoleInitSession: '/web/site/:siteId/console/node/:serverId/:login',
    consoleInitPodSession: '/web/site/:siteId/console/container/:serverId/namespace/:namespace/:pod/:container/:login',
    consoleSessionPlayer: '/web/site/:siteId/console/player/:sid',

    // sso redirects
    ssoOidcCallback: '/proxy/v1/webapi/*',
  },

  modules: {

    site: {
      defaultNamespace: 'default',
      features: {
        license: {
          enabled: false
        },
        logs: {
          enabled: true
        },
        k8s: {
          enabled: true
        },
        configMaps: {
          enabled: true
        },
        monitoring: {
          enabled: true,
          grafanaDefaultDashboardUrl: 'dashboard/db/cluster'
        }
      }
    },

    installer: {
      eulaAgreeText: 'I Agree To The Terms',
      eulaHeaderText: 'Welcome to the {0} Installer',
      eulaContentLabelText: 'License Agreement',
      licenseHeaderText: 'Enter your license',
      licenseOptionTrialText: 'Trial without license',
      licenseOptionText: 'With a license',
      licenseUserHintText: `If you have a license, please insert it here. In the next steps you will select the location of your application and the capacity you need`,
      progressUserHintText: 'Your infrastructure is being provisioned and your application is being installed.\n\n Once the installation is complete you will be taken to your infrastructure where you can access your application.',
      prereqUserHintText: `The cluster name will be used for issuing SSH and HTTP/TLS certificates to securely access the cluster.\n\n For this reason it is recommended to use a fully qualified domain name (FQDN) for the cluster name, e.g. prod.example.com`,
      provisionUserHintText: 'Drag the slider to estimate the number of resources needed for that performance level. You can also add / remove resources after the installation. \n\n Once you click "Start Installation" the resources will be provisioned on your infrastructure.',
      iamPermissionsHelpLink: 'https://gravitational.com/gravity/docs/overview/',
    }
  },

  api: {
    // portal
    appsPath: '/app/v1/applications/:repository?/:name?/:version?',
    licenseValidationPath: '/portalapi/v1/license/validate',
    standaloneInstallerPath: '/portalapi/v1/apps/:repository/:name/:version/installer',

    // provider
    providerPath: '/portalapi/v1/provider',

    // operations
    operationPath: '/portalapi/v1/sites/:siteId/operations/:opId?',
    operationProgressPath: '/portalapi/v1/sites/:siteId/operations/:opId/progress',
    operationAgentPath: '/portalapi/v1/sites/:siteId/operations/:opId/agent',
    operationStartPath: '/portalapi/v1/sites/:siteId/operations/:opId/start',
    operationPrecheckPath: '/portalapi/v1/sites/:siteId/operations/:opId/prechecks',
    operationLogsPath: `/portal/v1/accounts/${accountId}/sites/:siteId/operations/common/:opId/logs?access_token=:token`,
    shrinkSitePath: '/portalapi/v1/sites/:siteId/shrink',

    // auth & session management
    ssoOidc: '/proxy/v1/webapi/oidc/login/web?redirect_url=:redirect&connector_id=:providerName',
    ssoSaml: '/proxy/v1/webapi/saml/sso?redirect_url=:redirect&connector_id=:providerName',
    renewTokenPath: '/proxy/v1/webapi/sessions/renew',
    sessionPath: '/proxy/v1/webapi/sessions',
    u2fCreateUserChallengePath: '/proxy/v1/webapi/u2f/signuptokens/:inviteToken',
    u2fCreateUserPath: '/proxy/v1/webapi/u2f/users',
    u2fSessionChallengePath: '/proxy/v1/webapi/u2f/signrequest',
    u2fSessionPath: '/proxy/v1/webapi/u2f/sessions',

    // user management
    checkDomainNamePath: '/portalapi/v1/domains/:domainName',
    resetUserPath: '/portalapi/v1/sites/:siteId/users/:userId/reset',

    // user tokens
    userTokenInviteDonePath: '/portalapi/v1/tokens/invite/done',
    userTokenResetDonePath: '/portalapi/v1/tokens/reset/done',
    userTokenPath: '/portalapi/v1/tokens/user/:token',
    userStatusPath: '/portalapi/v1/user/status',

    // terminal
    ttyWsAddr: 'wss://:fqdm/proxy/v1/webapi/sites/:cluster/connect?access_token=:token&params=:params',
    ttyWsK8sPodAddr: 'wss://:fqdm/portalapi/v1/sites/:cluster/connect?access_token=:token&params=:params',
    scp: '/proxy/v1/webapi/sites/:siteId/nodes/:serverId/:login/scp?location=:location&filename=:filename',

    // site
    siteUserContextPath: '/portalapi/v1/sites/:siteId/context',
    siteTokenJoin: '/portalapi/v1/sites/:siteId/tokens/join',
    siteChangePasswordPath: '/portalapi/v1/sites/:siteId/users/password',
    siteUsersPath: '/portalapi/v1/sites/:siteId/users/:userId?',
    siteUserInvitePath: '/portalapi/v1/sites/:siteId/invites/:inviteId?',
    siteInfoPath: '/portalapi/v1/sites/:siteId/info',
    siteTlsCertPath: '/portalapi/v1/sites/:siteId/certificate',
    siteSessionPath: '/proxy/v1/webapi/sites/:siteId/sessions/:sid?',
    sitePath: '/portalapi/v1/sites/:siteId??shallow=:shallow',
    siteReportPath: '/portalapi/v1/sites/:siteId/report',
    siteEndpointsPath: '/portalapi/v1/sites/:siteId/endpoints',
    siteServersPath: '/portalapi/v1/sites/:siteId/servers',
    siteLogAggegatorPath: `/sites/v1/${accountId}/:siteId/proxy/master/logs/log?query=`,
    siteOperationReportPath: `/portal/v1/accounts/${accountId}/sites/:siteId/operations/common/:opId/crash-report`,
    siteAppsPath: '/portalapi/v1/sites/:siteId/releases',
    siteFlavorsPath: '/portalapi/v1/sites/:siteId/flavors',
    siteLicensePath: '/portalapi/v1/sites/:siteId/license',
    siteLogForwardersPath: '/portalapi/v1/sites/:siteId/logs/forwarders',
    siteMetricsPath: '/portalapi/v1/sites/:siteId/monitoring/metrics?interval=:interval&step=:step',
    siteRemoteAccessPath: '/portalapi/v1/sites/:siteId/access',
    siteGrafanaContextPath: '/portalapi/v1/sites/:siteId/grafana',
    siteResourcePath: '/portalapi/v1/sites/:siteId/resources/:kind?',
    siteRemoveResourcePath: '/portalapi/v1/sites/:siteId/resources/:kind/:id',
    siteEvents: `/proxy/v1/webapi/sites/:siteId/events/search?from=:start?&to=:end?&limit=:limit?`,
    siteSshSessionPath: '/proxy/v1/webapi/sites/:siteId/sessions/:sid?',

    // kubernetes
    k8sNamespacePath: `/sites/v1/${accountId}/:siteId/proxy/master/k8s/api/v1/namespaces`,
    k8sConfigMapsByNamespacePath: `/sites/v1/${accountId}/:siteId/proxy/master/k8s/api/v1/namespaces/:namespace/configmaps/:name`,
    k8sConfigMapsPath: `/sites/v1/${accountId}/:siteId/proxy/master/k8s/api/v1/configmaps`,
    k8sNodesPath: `/sites/v1/${accountId}/:siteId/proxy/master/k8s/api/v1/nodes`,
    k8sPodsByNamespacePath: `/sites/v1/${accountId}/:siteId/proxy/master/k8s/api/v1/namespaces/:namespace/pods`,
    k8sPodsPath: `/sites/v1/${accountId}/:siteId/proxy/master/k8s/api/v1/pods`,
    k8sSecretsPath: `/sites/v1/${accountId}/:siteId/proxy/master/k8s/api/v1/namespaces/:namespace/secrets/:name?`,
    k8sServicesPath: `/sites/v1/${accountId}/:siteId/proxy/master/k8s/api/v1/services`,
    k8sJobsPath: `/sites/v1/${accountId}/:siteId/proxy/master/k8s/apis/batch/v1/jobs`,
    k8sDelploymentsPath: `/sites/v1/${accountId}/:siteId/proxy/master/k8s/apis/extensions/v1beta1/deployments`,
    k8sDaemonSetsPath: `/sites/v1/${accountId}/:siteId/proxy/master/k8s/apis/extensions/v1beta1/daemonsets`,
  },

  getConsoleSessionRoute({ siteId, sid }) {
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.routes.consoleSession, { siteId, sid });
  },

  getConsolePlayerRoute({ siteId, sid }) {
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.routes.consoleSessionPlayer, { siteId, sid });
  },

  getConsoleInitSessionRoute({ siteId, login, serverId, sid }) {
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.routes.consoleInitSession, { siteId, serverId, login, sid });
  },

  getConsoleInitPodSessionRoute({ siteId, serverId, namespace, pod, container, login }) {
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.routes.consoleInitPodSession,
      { namespace, pod, container, login, serverId, siteId });
  },

  getSiteCertificateRoute(siteId) {
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.routes.siteCertificate, { siteId });
  },

  getSiteAuditRoute(siteId) {
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.routes.siteAudit, { siteId });
  },

  getSiteLicenseRoute(siteId) {
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.routes.siteLicense, { siteId });
  },

  getSiteUsersRoute(siteId) {
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.routes.siteUsers, { siteId });
  },

  getSiteSettingsRoute(siteId) {
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.routes.siteSettings, { siteId });
  },

  getSiteK8sPodMonitorRoute(siteId, namespace, podName) {
    return generatePath(cfg.routes.siteMonitorPod, { siteId, namespace, podName });
  },

  getSiteMonitorRoute(siteId) {
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.routes.siteMonitor, { siteId });
  },

  getSiteK8sRoute(namespace, category) {
    const siteId = cfg.defaultSiteId;
    return generatePath(cfg.routes.siteK8s, { siteId, namespace, category });
  },

  getSiteK8sConfigMapsRoute(namespace) {
    const siteId = cfg.defaultSiteId;
    return generatePath(cfg.routes.siteK8sConfigMaps, { siteId, namespace });
  },

  getSiteK8sSecretsRoute(namespace) {
    const siteId = cfg.defaultSiteId;
    return generatePath(cfg.routes.siteK8sSecrets, { siteId, namespace });
  },

  getSiteK8sJobsRoute(namespace) {
    const siteId = cfg.defaultSiteId;
    return generatePath(cfg.routes.siteK8sJobs, { siteId, namespace });
  },

  getSiteK8sPodsRoute(namespace) {
    const siteId = cfg.defaultSiteId;
    return generatePath(cfg.routes.siteK8sPods, { siteId, namespace });
  },

  getSiteK8sServicesRoute(namespace) {
    const siteId = cfg.defaultSiteId;
    return generatePath(cfg.routes.siteK8sServices, { siteId, namespace });
  },

  getSiteK8sDaemonsRoute(namespace) {
    const siteId = cfg.defaultSiteId;
    return generatePath(cfg.routes.siteK8sDaemonSets, { siteId, namespace });
  },

  getSiteK8sDeploymentsRoute(namespace) {
    const siteId = cfg.defaultSiteId;
    return generatePath(cfg.routes.siteK8sDeployments, { siteId, namespace });
  },

  getSiteLogsRoute(siteId) {
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.routes.siteLogs, { siteId });
  },

  getInstallNewSiteRoute(name, repository, version) {
    return generatePath(cfg.routes.installerApp,
      { name, repository, version });
  },

  getStandaloneInstallerPath(name, repository, version) {
    return generatePath(cfg.api.standaloneInstallerPath, { name, repository, version });
  },

  getSiteAppsUrl(siteId) {
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.api.siteAppsPath, { siteId });
  },

  getSiteSshSessionUrl({ siteId, sid } = {}) {
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.api.siteSshSessionPath, { siteId, sid });
  },

  getSiteServersUrl(siteId) {
    return generatePath(cfg.api.siteServersPath, { siteId });
  },

  getSiteMetricsUrl({ siteId, interval, step } = {}) {
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.api.siteMetricsPath, { siteId, interval, step });
  },

  getSiteResourcesUrl(kind) {
    const siteId = cfg.defaultSiteId;
    const path = cfg.api.siteResourcePath;
    return generatePath(path, { siteId, kind });
  },

  getSiteRemoveResourcesUrl(kind, id) {
    const siteId = cfg.defaultSiteId;
    return generatePath(cfg.api.siteRemoveResourcePath, { siteId, kind, id })
  },

  getSiteReportUrl() {
    const siteId = cfg.defaultSiteId;
    return generatePath(cfg.api.siteReportPath, { siteId });
  },

  getSiteInfoUrl(siteId) {
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.api.siteInfoPath, { siteId });
  },

  getSiteOperationReportUrl(siteId, opId) {
    return generatePath(cfg.api.siteOperationReportPath, {
      siteId,
      opId
    });
  },

  getSiteChangePasswordUrl(siteId) {
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.api.siteChangePasswordPath, { siteId });
  },

  getSiteUserUrl(siteId) {
    siteId = siteId || cfg.defaultSiteId;
    const url = generatePath(cfg.api.siteUsersPath, { siteId });
    return url.replace(/\/$/, "");
  },

  getSiteRemoteAccessUrl(siteId) {
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.api.siteRemoteAccessPath, { siteId });
  },

  getSiteEndpointsUrl(siteId) {
    return generatePath(cfg.api.siteEndpointsPath, { siteId });
  },

  getSiteLogForwardersUrl(siteId) {
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.api.siteLogForwardersPath, { siteId });
  },

  getSiteLogAggregatorUrl(siteId, query) {
    let path = cfg.api.siteLogAggegatorPath;
    if (query) {
      path = `${path}:query`;
    }

    return generatePath(path, { siteId, query });
  },

  getSiteTokenJoinUrl(siteId) {
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.api.siteTokenJoin, { siteId });
  },

  getShrinkSiteUrl(siteId) {
    return generatePath(cfg.api.shrinkSitePath, { siteId });
  },

  getAppsUrl(name, repository, version) {
    const url = generatePath(cfg.api.appsPath, { name, repository, version });
    return url.replace(/\/$/, '');
  },

  getOperationUrl({ siteId, opId }) {
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.api.operationPath, { siteId, opId }).replace(/\/$/g, '');
  },

  getOperationAgentUrl(siteId, opId) {
    return generatePath(cfg.api.operationAgentPath, { siteId, opId });
  },

  getOperationProgressUrl(siteId, opId) {
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.api.operationProgressPath, { siteId, opId });
  },

  getOperationStartUrl(siteId, opId) {
    return generatePath(cfg.api.operationStartPath, { siteId, opId });
  },

  operationPrecheckPath(siteId, opId) {
    return generatePath(cfg.api.operationPrecheckPath, { siteId, opId });
  },

  getSiteUrl({ siteId, shallow = true }) {
    return generatePath(cfg.api.sitePath, { siteId, shallow })
      .replace(/\/$/g, '')
      .replace(/\/\?shallow/g, '?shallow');
  },

  getSiteEventsUrl({ siteId, start, end, limit }) {
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.api.siteEvents, { siteId, start, end, limit });
  },

  getSiteGrafanaContextUrl(siteId) {
    return generatePath(cfg.api.siteGrafanaContextPath, { siteId });
  },

  getSiteLicenseUrl(siteId) {
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.api.siteLicensePath, { siteId });
  },

  getSiteFlavorsUrl(siteId) {
    return generatePath(cfg.api.siteFlavorsPath, { siteId });
  },

  getSiteUserContextUrl(siteId) {
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.api.siteUserContextPath, { siteId });
  },

  getInstallerProvisionUrl(siteId) {
    return generatePath(cfg.routes.installerCluster, { siteId });
  },

  getInstallerLastStepUrl(siteId) {
    return generatePath(cfg.routes.installerComplete, { siteId });
  },

  getCheckDomainNameUrl(domainName) {
    return generatePath(cfg.api.checkDomainNamePath, { domainName })
  },

  getAccountDeleteInviteUrl({ siteId, inviteId }) {
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.api.siteUserInvitePath, { siteId, inviteId });
  },

  getAccountDeleteUserUrl({ siteId, userId }) {
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.api.siteUsersPath, { siteId, userId });
  },

  getUserRequestInfo(token) {
    return generatePath(cfg.api.userTokenPath, { token });
  },

  getSsoUrl(providerUrl, providerName, redirect) {
    return cfg.baseUrl + "/proxy" + generatePath(providerUrl, { redirect, providerName });
  },

  getAuth2faType() {
    let [secondFactor = null] = at(cfg, 'auth.second_factor');
    return secondFactor;
  },

  getU2fCreateUserChallengeUrl(inviteToken) {
    return generatePath(cfg.api.u2fCreateUserChallengePath, { inviteToken });
  },

  getAuthProviders() {
    return cfg.auth && cfg.auth.providers ? cfg.auth.providers : [];
  },

  getSiteRoute(siteId) {
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.routes.siteBase, { siteId });
  },

  getSiteServersRoute(siteId) {
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.routes.siteServers, { siteId });
  },

  getSiteLogQueryRoute({ siteId, query }) {
    siteId = siteId || cfg.defaultSiteId;
    const route = cfg.routes.siteLogs;
    return generatePath(`${route}?query=:query`, { siteId, query });
  },

  getSiteTlsCertUrl(siteId) {
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.api.siteTlsCertPath, { siteId });
  },

  getSiteUserInvitePath(siteId) {
    siteId = siteId || cfg.defaultSiteId;
    const url = generatePath(cfg.api.siteUserInvitePath, { siteId });
    return url.replace(/\/$/, '');
  },

  getSiteUserResetPath({ siteId, userId }) {
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.api.resetUserPath, { siteId, userId });
  },

  getSiteSessionUrl(siteId, sid) {
    return generatePath(cfg.api.siteSessionPath, { siteId, sid });
  },

  getSiteInstallerRoute(siteId) {
    return generatePath(cfg.routes.installerCluster, { siteId });
  },

  getSiteDefaultDashboard() {
    let [suffix] = at(cfg, 'modules.site.features.monitoring.grafanaDefaultDashboardUrl');
    return suffix;
  },

  getAgentDeviceMount(name) {
    const [option] = at(cfg, `agentReport.provision.mounts.${name}`);
    return option || {};
  },

  getAgentDeviceIpv4() {
    const [option] = at(cfg, 'agentReport.provision.interfaces.ipv4');
    return option || {};
  },

  getScpUrl({ siteId, serverId, login, location, filename }) {
    return generatePath(cfg.api.scp, { siteId, serverId, login, location, filename });
  },

  enableSiteLicense(value = false) {
    cfg.modules.site.features.license.enabled = value;
  },

  enableSiteMonitoring(value = true) {
    cfg.modules.site.features.monitoring.enabled = value;
  },

  enableSiteK8s(value = true) {
    cfg.modules.site.features.k8s.enabled = value;
  },

  enableSiteConfigMaps(value = true) {
    cfg.modules.site.features.configMaps.enabled = value;
  },

  enableSiteLogs(value = true) {
    cfg.modules.site.features.logs.enabled = value;
  },

  isSiteMonitoringEnabled() {
    return cfg.modules.site.features.monitoring.enabled;
  },

  isSiteLicenseEnabled() {
    return cfg.modules.site.features.license.enabled;
  },

  isSiteK8sEnabled() {
    return cfg.modules.site.features.k8s.enabled;
  },

  isSiteConfigMapsEnabled() {
    return cfg.modules.site.features.configMaps.enabled;
  },

  isSiteLogsEnabled() {
    return cfg.modules.site.features.logs.enabled;
  },

  /**
 * getLocalSiteId returns local cluster id.
 * for remotely accessing clusters this will always be HUB cluster id.
 */
  getLocalSiteId() {
    const [siteId] = at(cfg, 'systemInfo.clusterName');
    return siteId;
  },

  setDefaultSiteId(siteId) {
    this.defaultSiteId = siteId;
  },

  setLogo(logo) {
    this.logo = logo;
  },

  init(config = {}) {
    $.extend(true, this, config);
  }
}

export default cfg;
