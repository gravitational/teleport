/*
Copyright 2015-2021 Gravitational, Inc.

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

import { generatePath } from 'react-router';
import { mergeDeep } from 'shared/utils/highbar';

import generateResourcePath from './generateResourcePath';

import type {
  Auth2faType,
  AuthProvider,
  AuthType,
  PreferredMfaType,
  PrimaryAuthType,
  PrivateKeyPolicy,
} from 'shared/services';

import type { SortType } from 'teleport/services/agents';
import type { RecordingType } from 'teleport/services/recordings';
import type { WebauthnAssertionResponse } from './services/auth';

import type { ParticipantMode } from 'teleport/services/session';

const cfg = {
  isEnterprise: false,
  isCloud: false,
  assistEnabled: false,
  automaticUpgrades: false,
  isDashboard: false,
  tunnelPublicAddress: '',
  recoveryCodesEnabled: false,
  // IsUsageBasedBilling determines if the user subscription is usage-based (pay-as-you-go).
  isUsageBasedBilling: false,

  configDir: '$HOME/.config',

  baseUrl: window.location.origin,

  ui: {
    scrollbackLines: 1000,
  },

  auth: {
    localAuthEnabled: true,
    allowPasswordless: false,
    // localConnectorName is used to determine primary "local" auth preference.
    // Currently, there is only one bookmarked connector name "passwordless"
    // that when used, passwordless is the preferred authn method. Empty means
    // local user + password authn preference.
    localConnectorName: '',
    providers: [] as AuthProvider[],
    second_factor: 'off' as Auth2faType,
    authType: 'local' as AuthType,
    preferredLocalMfa: '' as PreferredMfaType,
    privateKeyPolicy: 'none' as PrivateKeyPolicy,
    // MOTD is message of the day, displayed to users before login.
    motd: '',
  },

  proxyCluster: 'localhost',

  loc: {
    dateTimeFormat: 'YYYY-MM-DD HH:mm:ss',
    dateFormat: 'YYYY-MM-DD',
  },

  routes: {
    root: '/web',
    discover: '/web/discover',
    assistBase: '/web/assist/',
    assist: '/web/assist/:conversationId?',
    apps: '/web/cluster/:clusterId/apps',
    appLauncher: '/web/launch/:fqdn/:clusterId?/:publicAddr?/:arn?',
    support: '/web/support',
    settings: '/web/settings',
    account: '/web/account',
    accountPassword: '/web/account/password',
    accountMfaDevices: '/web/account/twofactor',
    roles: '/web/roles',
    sso: '/web/sso',
    cluster: '/web/cluster/:clusterId/',
    clusters: '/web/clusters',
    trustedClusters: '/web/trust',
    audit: '/web/cluster/:clusterId/audit',
    nodes: '/web/cluster/:clusterId/nodes',
    sessions: '/web/cluster/:clusterId/sessions',
    recordings: '/web/cluster/:clusterId/recordings',
    databases: '/web/cluster/:clusterId/databases',
    desktops: '/web/cluster/:clusterId/desktops',
    desktop: '/web/cluster/:clusterId/desktops/:desktopName/:username',
    users: '/web/users',
    console: '/web/cluster/:clusterId/console',
    consoleNodes: '/web/cluster/:clusterId/console/nodes',
    consoleConnect: '/web/cluster/:clusterId/console/node/:serverId/:login',
    consoleSession: '/web/cluster/:clusterId/console/session/:sid',
    player: '/web/cluster/:clusterId/session/:sid', // ?recordingType=ssh|desktop|k8s&durationMs=1234
    login: '/web/login',
    loginSuccess: '/web/msg/info/login_success',
    loginErrorLegacy: '/web/msg/error/login_failed',
    loginError: '/web/msg/error/login',
    loginErrorCallback: '/web/msg/error/login/callback',
    loginErrorUnauthorized: '/web/msg/error/login/auth',
    userInvite: '/web/invite/:tokenId',
    userInviteContinue: '/web/invite/:tokenId/continue',
    userReset: '/web/reset/:tokenId',
    userResetContinue: '/web/reset/:tokenId/continue',
    kubernetes: '/web/cluster/:clusterId/kubernetes',
    headlessSso: `/web/headless/:requestId`,
    integrations: '/web/integrations',
    integrationEnroll: '/web/integrations/new/:type?',
    locks: '/web/locks',
    newLock: '/web/locks/new',

    // whitelist sso handlers
    oidcHandler: '/v1/webapi/oidc/*',
    samlHandler: '/v1/webapi/saml/*',
    githubHandler: '/v1/webapi/github/*',
  },

  api: {
    appSession: '/v1/webapi/sessions/app',
    appFqdnPath: '/v1/webapi/apps/:fqdn/:clusterId?/:publicAddr?',
    applicationsPath:
      '/v1/webapi/sites/:clusterId/apps?searchAsRoles=:searchAsRoles?&limit=:limit?&startKey=:startKey?&query=:query?&search=:search?&sort=:sort?',
    clustersPath: '/v1/webapi/sites',
    clusterAlertsPath: '/v1/webapi/sites/:clusterId/alerts',
    clusterEventsPath: `/v1/webapi/sites/:clusterId/events/search?from=:start?&to=:end?&limit=:limit?&startKey=:startKey?&include=:include?`,
    clusterEventsRecordingsPath: `/v1/webapi/sites/:clusterId/events/search/sessions?from=:start?&to=:end?&limit=:limit?&startKey=:startKey?`,

    connectionDiagnostic: `/v1/webapi/sites/:clusterId/diagnostics/connections`,
    checkAccessToRegisteredResource: `/v1/webapi/sites/:clusterId/resources/check`,

    scp: '/v1/webapi/sites/:clusterId/nodes/:serverId/:login/scp?location=:location&filename=:filename&moderatedSessionId=:moderatedSessionId?&fileTransferRequestId=:fileTransferRequestId?',
    webRenewTokenPath: '/v1/webapi/sessions/web/renew',
    resetPasswordTokenPath: '/v1/webapi/users/password/token',
    webSessionPath: '/v1/webapi/sessions/web',
    userContextPath: '/v1/webapi/sites/:clusterId/context',
    userStatusPath: '/v1/webapi/user/status',
    passwordTokenPath: '/v1/webapi/users/password/token/:tokenId?',
    changeUserPasswordPath: '/v1/webapi/users/password',
    nodesPath:
      '/v1/webapi/sites/:clusterId/nodes?searchAsRoles=:searchAsRoles?&limit=:limit?&startKey=:startKey?&query=:query?&search=:search?&sort=:sort?',

    databaseServicesPath: `/v1/webapi/sites/:clusterId/databaseservices`,
    databaseIamPolicyPath: `/v1/webapi/sites/:clusterId/databases/:database/iam/policy`,
    databasePath: `/v1/webapi/sites/:clusterId/databases/:database`,
    databasesPath: `/v1/webapi/sites/:clusterId/databases?searchAsRoles=:searchAsRoles?&limit=:limit?&startKey=:startKey?&query=:query?&search=:search?&sort=:sort?`,

    desktopsPath: `/v1/webapi/sites/:clusterId/desktops?searchAsRoles=:searchAsRoles?&limit=:limit?&startKey=:startKey?&query=:query?&search=:search?&sort=:sort?`,
    desktopServicesPath: `/v1/webapi/sites/:clusterId/desktopservices?searchAsRoles=:searchAsRoles?&limit=:limit?&startKey=:startKey?&query=:query?&search=:search?&sort=:sort?`,
    desktopPath: `/v1/webapi/sites/:clusterId/desktops/:desktopName`,
    desktopWsAddr:
      'wss://:fqdn/v1/webapi/sites/:clusterId/desktops/:desktopName/connect?access_token=:token&username=:username&width=:width&height=:height',
    desktopPlaybackWsAddr:
      'wss://:fqdn/v1/webapi/sites/:clusterId/desktopplayback/:sid?access_token=:token',
    desktopIsActive: '/v1/webapi/sites/:clusterId/desktops/:desktopName/active',
    ttyWsAddr:
      'wss://:fqdn/v1/webapi/sites/:clusterId/connect?access_token=:token&params=:params&traceparent=:traceparent',
    activeAndPendingSessionsPath: '/v1/webapi/sites/:clusterId/sessions',
    sshPlaybackPrefix: '/v1/webapi/sites/:clusterId/sessions/:sid', // prefix because this is eventually concatenated with "/stream" or "/events"
    kubernetesPath:
      '/v1/webapi/sites/:clusterId/kubernetes?searchAsRoles=:searchAsRoles?&limit=:limit?&startKey=:startKey?&query=:query?&search=:search?&sort=:sort?',

    usersPath: '/v1/webapi/users',
    userWithUsernamePath: '/v1/webapi/users/:username',
    createPrivilegeTokenPath: '/v1/webapi/users/privilege/token',

    rolesPath: '/v1/webapi/roles/:name?',
    githubConnectorsPath: '/v1/webapi/github/:name?',
    trustedClustersPath: '/v1/webapi/trustedcluster/:name?',

    joinTokenPath: '/v1/webapi/token',
    dbScriptPath: '/scripts/:token/install-database.sh',
    nodeScriptPath: '/scripts/:token/install-node.sh',
    appNodeScriptPath: '/scripts/:token/install-app.sh?name=:name&uri=:uri',

    mfaRequired: '/v1/webapi/sites/:clusterId/mfa/required',
    mfaLoginBegin: '/v1/webapi/mfa/login/begin', // creates authnenticate challenge with user and password
    mfaLoginFinish: '/v1/webapi/mfa/login/finishsession', // creates a web session
    mfaChangePasswordBegin: '/v1/webapi/mfa/authenticatechallenge/password',

    headlessSsoPath: `/v1/webapi/headless/:requestId`,

    mfaCreateRegistrationChallengePath:
      '/v1/webapi/mfa/token/:tokenId/registerchallenge',

    mfaRegisterChallengeWithTokenPath:
      '/v1/webapi/mfa/token/:tokenId/registerchallenge',
    mfaAuthnChallengePath: '/v1/webapi/mfa/authenticatechallenge',
    mfaAuthnChallengeWithTokenPath:
      '/v1/webapi/mfa/token/:tokenId/authenticatechallenge',
    mfaDevicesWithTokenPath: '/v1/webapi/mfa/token/:tokenId/devices',
    mfaDevicesPath: '/v1/webapi/mfa/devices',
    mfaDevicePath: '/v1/webapi/mfa/token/:tokenId/devices/:deviceName',

    locksPath: '/v1/webapi/sites/:clusterId/locks',
    locksPathWithUuid: '/v1/webapi/sites/:clusterId/locks/:uuid',

    dbSign: 'v1/webapi/sites/:clusterId/sign/db',

    installADDSPath: '/v1/webapi/scripts/desktop-access/install-ad-ds.ps1',
    installADCSPath: '/v1/webapi/scripts/desktop-access/install-ad-cs.ps1',
    configureADPath:
      '/v1/webapi/scripts/desktop-access/configure/:token/configure-ad.ps1',

    captureUserEventPath: '/v1/webapi/capture',
    capturePreUserEventPath: '/v1/webapi/precapture',

    webapiPingPath: '/v1/webapi/ping',

    headlessLogin: '/v1/webapi/headless/:headless_authentication_id',

    integrationsPath: '/v1/webapi/sites/:clusterId/integrations/:name?',
    thumbprintPath: '/v1/webapi/thumbprint',
    awsRdsDbListPath:
      '/v1/webapi/sites/:clusterId/integrations/aws-oidc/:name/databases',

    userGroupsListPath:
      '/v1/webapi/sites/:clusterId/user-groups?searchAsRoles=:searchAsRoles?&limit=:limit?&startKey=:startKey?&query=:query?&search=:search?&sort=:sort?',

    assistConversationsPath: '/v1/webapi/assistant/conversations',
    assistSetConversationTitlePath:
      '/v1/webapi/assistant/conversations/:conversationId/title',
    assistGenerateSummaryPath: '/v1/webapi/assistant/title/summary',
    assistConversationWebSocketPath:
      'wss://:hostname/v1/webapi/sites/:clusterId/assistant',
    assistConversationHistoryPath:
      '/v1/webapi/assistant/conversations/:conversationId',
    assistExecuteCommandWebSocketPath:
      'wss://:hostname/v1/webapi/command/:clusterId/execute',
  },

  getAppFqdnUrl(params: UrlAppParams) {
    return generatePath(cfg.api.appFqdnPath, { ...params });
  },

  getClusterAlertsUrl(clusterId: string) {
    return generatePath(cfg.api.clusterAlertsPath, {
      clusterId,
    });
  },

  getClusterEventsUrl(clusterId: string, params: UrlClusterEventsParams) {
    return generatePath(cfg.api.clusterEventsPath, {
      clusterId,
      ...params,
    });
  },

  getClusterEventsRecordingsUrl(
    clusterId: string,
    params: UrlSessionRecordingsParams
  ) {
    return generatePath(cfg.api.clusterEventsRecordingsPath, {
      clusterId,
      ...params,
    });
  },

  getAuthProviders() {
    return cfg.auth && cfg.auth.providers ? cfg.auth.providers : [];
  },

  getAuth2faType() {
    return cfg.auth ? cfg.auth.second_factor : null;
  },

  getPreferredMfaType() {
    return cfg.auth ? cfg.auth.preferredLocalMfa : null;
  },

  getMOTD() {
    return cfg.auth ? cfg.auth.motd : null;
  },

  getLocalAuthFlag() {
    return cfg.auth.localAuthEnabled;
  },

  getPrivateKeyPolicy() {
    return cfg.auth.privateKeyPolicy;
  },

  isPasswordlessEnabled() {
    return cfg.auth.allowPasswordless;
  },

  getPrimaryAuthType(): PrimaryAuthType {
    if (cfg.auth.localConnectorName === 'passwordless') {
      return 'passwordless';
    }

    if (cfg.auth.authType === 'local') return 'local';

    return 'sso';
  },

  getAuthType() {
    return cfg.auth.authType;
  },

  getSsoUrl(providerUrl, providerName, redirect) {
    return cfg.baseUrl + generatePath(providerUrl, { redirect, providerName });
  },

  getAuditRoute(clusterId: string) {
    return generatePath(cfg.routes.audit, { clusterId });
  },

  getIntegrationEnrollRoute(type?: string) {
    return generatePath(cfg.routes.integrationEnroll, { type });
  },

  getNodesRoute(clusterId: string) {
    return generatePath(cfg.routes.nodes, { clusterId });
  },

  getDatabasesRoute(clusterId: string) {
    return generatePath(cfg.routes.databases, { clusterId });
  },

  getDesktopsRoute(clusterId: string) {
    return generatePath(cfg.routes.desktops, { clusterId });
  },

  getJoinTokenUrl() {
    return cfg.api.joinTokenPath;
  },

  getNodeScriptUrl(token: string) {
    return cfg.baseUrl + generatePath(cfg.api.nodeScriptPath, { token });
  },

  getDbScriptUrl(token: string) {
    return cfg.baseUrl + generatePath(cfg.api.dbScriptPath, { token });
  },

  getConfigureADUrl(token: string) {
    return cfg.baseUrl + generatePath(cfg.api.configureADPath, { token });
  },

  getInstallADDSPath() {
    return cfg.baseUrl + cfg.api.installADDSPath;
  },

  getInstallADCSPath() {
    return cfg.baseUrl + cfg.api.installADCSPath;
  },

  getAppNodeScriptUrl(token: string, name: string, uri: string) {
    return (
      cfg.baseUrl +
      generatePath(cfg.api.appNodeScriptPath, { token, name, uri })
    );
  },

  getUsersRoute() {
    const clusterId = cfg.proxyCluster;
    return generatePath(cfg.routes.users, { clusterId });
  },

  getAppsRoute(clusterId: string) {
    return generatePath(cfg.routes.apps, { clusterId });
  },

  getSessionsRoute(clusterId: string) {
    return generatePath(cfg.routes.sessions, { clusterId });
  },

  getRecordingsRoute(clusterId: string) {
    return generatePath(cfg.routes.recordings, { clusterId });
  },

  getConsoleNodesRoute(clusterId: string) {
    return generatePath(cfg.routes.consoleNodes, {
      clusterId,
    });
  },

  getSshConnectRoute({ clusterId, login, serverId }: UrlParams) {
    return generatePath(cfg.routes.consoleConnect, {
      clusterId,
      serverId,
      login,
    });
  },

  getDesktopRoute({ clusterId, username, desktopName }) {
    return generatePath(cfg.routes.desktop, {
      clusterId,
      desktopName,
      username,
    });
  },

  getSshSessionRoute({ clusterId, sid }: UrlParams, mode?: ParticipantMode) {
    const basePath = generatePath(cfg.routes.consoleSession, {
      clusterId,
      sid,
    });
    if (mode) {
      return `${basePath}?mode=${mode}`;
    }
    return basePath;
  },

  getPasswordTokenUrl(tokenId?: string) {
    return generatePath(cfg.api.passwordTokenPath, { tokenId });
  },

  getClusterRoute(clusterId: string) {
    return generatePath(cfg.routes.cluster, { clusterId });
  },

  getConsoleRoute(clusterId: string) {
    return generatePath(cfg.routes.console, { clusterId });
  },

  getAppLauncherRoute(params: UrlLauncherParams) {
    return generatePath(cfg.routes.appLauncher, { ...params });
  },

  getPlayerRoute(params: UrlPlayerParams, search: UrlPlayerSearch) {
    let route = generatePath(cfg.routes.player, { ...params });
    route = `${route}?recordingType=${search.recordingType}`;

    if (search.durationMs) {
      route = `${route}&durationMs=${search.durationMs}`;
    }

    return route;
  },

  getConnectionDiagnosticUrl() {
    const clusterId = cfg.proxyCluster;
    return generatePath(cfg.api.connectionDiagnostic, { clusterId });
  },

  getMfaRequiredUrl() {
    const clusterId = cfg.proxyCluster;
    return generatePath(cfg.api.mfaRequired, { clusterId });
  },

  getCheckAccessToRegisteredResourceUrl() {
    const clusterId = cfg.proxyCluster;
    return generatePath(cfg.api.checkAccessToRegisteredResource, {
      clusterId,
    });
  },

  getUserContextUrl() {
    const clusterId = cfg.proxyCluster;
    return generatePath(cfg.api.userContextPath, { clusterId });
  },

  getUserResetTokenRoute(tokenId = '', invite = true) {
    const route = invite ? cfg.routes.userInvite : cfg.routes.userReset;
    return cfg.baseUrl + generatePath(route, { tokenId });
  },

  getUserResetTokenContinueRoute(tokenId = '') {
    return generatePath(cfg.routes.userResetContinue, { tokenId });
  },

  getHeadlessSsoPath(requestId: string) {
    return generatePath(cfg.api.headlessSsoPath, { requestId });
  },

  getUserInviteTokenRoute(tokenId = '') {
    return generatePath(cfg.routes.userInvite, { tokenId });
  },

  getUserInviteTokenContinueRoute(tokenId = '') {
    return generatePath(cfg.routes.userInviteContinue, { tokenId });
  },

  getKubernetesRoute(clusterId: string) {
    return generatePath(cfg.routes.kubernetes, { clusterId });
  },

  getUsersUrl() {
    return cfg.api.usersPath;
  },

  getUserWithUsernameUrl(username: string) {
    return generatePath(cfg.api.userWithUsernamePath, { username });
  },

  getSshPlaybackPrefixUrl({ clusterId, sid }: UrlParams) {
    return generatePath(cfg.api.sshPlaybackPrefix, { clusterId, sid });
  },

  getActiveAndPendingSessionsUrl({ clusterId }: UrlParams) {
    return generatePath(cfg.api.activeAndPendingSessionsPath, { clusterId });
  },

  getClusterNodesUrl(clusterId: string, params: UrlResourcesParams) {
    return generateResourcePath(cfg.api.nodesPath, {
      clusterId,
      ...params,
    });
  },

  getDatabaseServicesUrl(clusterId: string) {
    return generatePath(cfg.api.databaseServicesPath, {
      clusterId,
    });
  },

  getDatabaseIamPolicyUrl(clusterId: string, dbName: string) {
    return generatePath(cfg.api.databaseIamPolicyPath, {
      clusterId,
      database: dbName,
    });
  },

  getDatabaseUrl(clusterId: string, dbName: string) {
    return generatePath(cfg.api.databasePath, {
      clusterId,
      database: dbName,
    });
  },

  getDatabasesUrl(clusterId: string, params?: UrlResourcesParams) {
    return generateResourcePath(cfg.api.databasesPath, {
      clusterId,
      ...params,
    });
  },

  getLocksRoute() {
    return cfg.routes.locks;
  },

  getNewLocksRoute() {
    return cfg.routes.newLock;
  },

  getLocksUrl() {
    // Currently only support get/create locks in root cluster.
    const clusterId = cfg.proxyCluster;
    return generatePath(cfg.api.locksPath, { clusterId });
  },

  getLocksUrlWithUuid(uuid: string) {
    // Currently only support delete/lookup locks in root cluster.
    const clusterId = cfg.proxyCluster;
    return generatePath(cfg.api.locksPathWithUuid, { clusterId, uuid });
  },

  getDatabaseSignUrl(clusterId: string) {
    return generatePath(cfg.api.dbSign, { clusterId });
  },

  getDesktopsUrl(clusterId: string, params: UrlResourcesParams) {
    return generateResourcePath(cfg.api.desktopsPath, {
      clusterId,
      ...params,
    });
  },

  getDesktopServicesUrl(clusterId: string, params: UrlResourcesParams) {
    return generateResourcePath(cfg.api.desktopServicesPath, {
      clusterId,
      ...params,
    });
  },

  getDesktopUrl(clusterId: string, desktopName: string) {
    return generatePath(cfg.api.desktopPath, { clusterId, desktopName });
  },

  getDesktopIsActiveUrl(clusterId: string, desktopName: string) {
    return generatePath(cfg.api.desktopIsActive, { clusterId, desktopName });
  },

  getApplicationsUrl(clusterId: string, params: UrlResourcesParams) {
    return generateResourcePath(cfg.api.applicationsPath, {
      clusterId,
      ...params,
    });
  },

  getScpUrl({ webauthn, ...params }: UrlScpParams) {
    let path = generatePath(cfg.api.scp, {
      ...params,
    });

    if (!webauthn) {
      return path;
    }
    // non-required MFA will mean this param is undefined and generatePath doesn't like undefined
    // or optional params. So we append it ourselves here. Its ok to be undefined when sent to the server
    // as the existence of this param is what will issue certs
    return `${path}&webauthn=${JSON.stringify({
      webauthnAssertionResponse: webauthn,
    })}`;
  },

  getRenewTokenUrl() {
    return cfg.api.webRenewTokenPath;
  },

  getGithubConnectorsUrl(name?: string) {
    return generatePath(cfg.api.githubConnectorsPath, { name });
  },

  getTrustedClustersUrl(name?: string) {
    return generatePath(cfg.api.trustedClustersPath, { name });
  },

  getRolesUrl(name?: string) {
    return generatePath(cfg.api.rolesPath, { name });
  },

  getKubernetesUrl(clusterId: string, params: UrlResourcesParams) {
    return generateResourcePath(cfg.api.kubernetesPath, {
      clusterId,
      ...params,
    });
  },

  getAuthnChallengeWithTokenUrl(tokenId: string) {
    return generatePath(cfg.api.mfaAuthnChallengeWithTokenPath, {
      tokenId,
    });
  },

  getMfaDevicesWithTokenUrl(tokenId: string) {
    return generatePath(cfg.api.mfaDevicesWithTokenPath, { tokenId });
  },

  getMfaDeviceUrl(tokenId: string, deviceName: string) {
    return generatePath(cfg.api.mfaDevicePath, { tokenId, deviceName });
  },

  getMfaCreateRegistrationChallengeUrl(tokenId: string) {
    return generatePath(cfg.api.mfaCreateRegistrationChallengePath, {
      tokenId,
    });
  },

  getIntegrationsUrl(integrationName?: string) {
    // Currently you can only create integrations at the root cluster.
    const clusterId = cfg.proxyCluster;
    return generatePath(cfg.api.integrationsPath, {
      clusterId,
      name: integrationName,
    });
  },

  getAwsRdsDbListUrl(integrationName: string) {
    const clusterId = cfg.proxyCluster;

    return generatePath(cfg.api.awsRdsDbListPath, {
      clusterId,
      name: integrationName,
    });
  },

  getUserGroupsListUrl(clusterId: string, params: UrlResourcesParams) {
    return generateResourcePath(cfg.api.userGroupsListPath, {
      clusterId,
      ...params,
    });
  },

  getUIConfig() {
    return cfg.ui;
  },

  getAssistSetConversationTitleUrl(conversationId: string) {
    return generatePath(cfg.api.assistSetConversationTitlePath, {
      conversationId,
    });
  },

  getAssistConversationWebSocketUrl(
    hostname: string,
    clusterId: string,
    accessToken: string,
    conversationId: string
  ) {
    const searchParams = new URLSearchParams();

    searchParams.set('access_token', accessToken);
    searchParams.set('conversation_id', conversationId);

    return (
      generatePath(cfg.api.assistConversationWebSocketPath, {
        hostname,
        clusterId,
      }) + `?${searchParams.toString()}`
    );
  },

  getAssistConversationHistoryUrl(conversationId: string) {
    return generatePath(cfg.api.assistConversationHistoryPath, {
      conversationId,
    });
  },

  getAssistExecuteCommandUrl(
    hostname: string,
    clusterId: string,
    accessToken: string,
    params: Record<string, string>
  ) {
    const searchParams = new URLSearchParams();

    searchParams.set('access_token', accessToken);
    searchParams.set('params', JSON.stringify(params));

    return (
      generatePath(cfg.api.assistExecuteCommandWebSocketPath, {
        hostname,
        clusterId,
      }) + `?${searchParams.toString()}`
    );
  },

  getAssistConversationUrl(conversationId: string) {
    return generatePath(cfg.routes.assist, { conversationId });
  },

  init(backendConfig = {}) {
    mergeDeep(this, backendConfig);
  },
};

export interface UrlParams {
  clusterId: string;
  sid?: string;
  login?: string;
  serverId?: string;
}

export interface UrlAppParams {
  fqdn: string;
  clusterId?: string;
  publicAddr?: string;
  arn?: string;
}

export interface UrlScpParams {
  clusterId: string;
  serverId: string;
  login: string;
  location: string;
  filename: string;
  moderatedSessionId?: string;
  fileTransferRequestId?: string;
  webauthn?: WebauthnAssertionResponse;
}

export interface UrlSshParams {
  login?: string;
  serverId?: string;
  sid?: string;
  mode?: ParticipantMode;
  clusterId: string;
}

export interface UrlSessionRecordingsParams {
  start: string;
  end: string;
  limit?: number;
  startKey?: string;
}

export interface UrlClusterEventsParams {
  start: string;
  end: string;
  limit?: number;
  include?: string;
  startKey?: string;
}

export interface UrlLauncherParams {
  fqdn: string;
  clusterId?: string;
  publicAddr?: string;
  arn?: string;
}

export interface UrlPlayerParams {
  clusterId: string;
  sid: string;
}

export interface UrlPlayerSearch {
  recordingType: RecordingType;
  durationMs?: number; // this is only necessary for recordingType == desktop
}

// /web/cluster/:clusterId/desktops/:desktopName/:username
export interface UrlDesktopParams {
  username?: string;
  desktopName?: string;
  clusterId: string;
}

export interface UrlResourcesParams {
  query?: string;
  search?: string;
  sort?: SortType;
  limit?: number;
  startKey?: string;
  searchAsRoles?: 'yes' | '';
}

export interface UrlIntegrationExecuteRequestParams {
  // name is the name of integration to execute (use).
  name: string;
  // action is the expected backend string value
  // used to describe what to use the integration for.
  action: 'aws-oidc/list_databases';
}

export default cfg;
