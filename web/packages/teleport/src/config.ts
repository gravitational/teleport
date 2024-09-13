/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { generatePath } from 'react-router';
import { mergeDeep } from 'shared/utils/highbar';
import { IncludedResourceMode } from 'shared/components/UnifiedResources';

import generateResourcePath from './generateResourcePath';

import { defaultEntitlements } from './entitlement';

import type {
  Auth2faType,
  AuthProvider,
  AuthType,
  PreferredMfaType,
  PrimaryAuthType,
} from 'shared/services';

import type { SortType } from 'teleport/services/agents';
import type { RecordingType } from 'teleport/services/recordings';
import type { WebauthnAssertionResponse } from './services/auth';
import type { PluginKind, Regions } from './services/integrations';
import type { ParticipantMode } from 'teleport/services/session';
import type { YamlSupportedResourceKind } from './services/yaml/types';

const cfg = {
  /** @deprecated Use cfg.edition instead. */
  isEnterprise: false,
  edition: 'oss',
  isCloud: false,
  automaticUpgrades: false,
  automaticUpgradesTargetVersion: '',
  // isDashboard is used generally when we want to hide features that can't be hidden by RBAC in
  // the case of a self-hosted license tenant dashboard.
  isDashboard: false,
  tunnelPublicAddress: '',
  recoveryCodesEnabled: false,
  // IsUsageBasedBilling determines if the user subscription is usage-based (pay-as-you-go).
  // Historically, this flag used to refer to "Cloud Team" product,
  // but with the new EUB (Enterprise Usage Based) product, it can mean either EUB or Team.
  // Prefer using feature flags to determine if something should be enabled or not.
  // If you have no other options, use the `isStripeManaged` config flag to determine if product used is Team.
  // EUB can be determined from a combination of existing config flags eg: `isUsageBasedBilling && !isStripeManaged`.
  isUsageBasedBilling: false,
  hideInaccessibleFeatures: false,
  customTheme: '',
  /** @deprecated */
  isTeam: false,
  isStripeManaged: false,
  hasQuestionnaire: false,
  externalAuditStorage: false,
  premiumSupport: false,
  accessRequests: false,
  /** @deprecated Use entitlements instead. */
  trustedDevices: false,
  oidc: false,
  saml: false,
  /** @deprecated Use entitlements instead. */
  joinActiveSessions: false,
  /** @deprecated Use entitlements instead. */
  mobileDeviceManagement: false,
  /** @deprecated Use entitlements instead. */
  isIgsEnabled: false,

  // isPolicyEnabled refers to the Teleport Policy product
  isPolicyEnabled: false,

  configDir: '$HOME/.config',

  baseUrl: window.location.origin,

  // enterprise non-exact routes will be merged into this
  // see `getNonExactRoutes` for details about non-exact routes
  nonExactRoutes: [],

  // featureLimits define limits for features.
  /** @deprecated Use entitlements instead. */
  featureLimits: {
    /** @deprecated Use entitlements instead. */
    accessListCreateLimit: 0,
    /** @deprecated Use entitlements instead. */
    accessMonitoringMaxReportRangeLimit: 0,
    /** @deprecated Use entitlements instead. */
    AccessRequestMonthlyRequestLimit: 0,
  },

  // default entitlements to false
  entitlements: defaultEntitlements,

  ui: {
    scrollbackLines: 1000,
    showResources: 'requestable',
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
    // motd is the message of the day, displayed to users before login.
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
    accessRequest: '/web/accessrequest',
    apps: '/web/cluster/:clusterId/apps',
    appLauncher: '/web/launch/:fqdn/:clusterId?/:publicAddr?/:arn?',
    support: '/web/support',
    settings: '/web/settings',
    account: '/web/account',
    accountPassword: '/web/account/password',
    accountMfaDevices: '/web/account/twofactor',
    roles: '/web/roles',
    joinTokens: '/web/tokens',
    deviceTrust: `/web/devices`,
    deviceTrustAuthorize: '/web/device/authorize/:id?/:token?',
    sso: '/web/sso',
    cluster: '/web/cluster/:clusterId/',
    clusters: '/web/clusters',
    trustedClusters: '/web/trust',
    audit: '/web/cluster/:clusterId/audit',
    unifiedResources: '/web/cluster/:clusterId/resources',
    nodes: '/web/cluster/:clusterId/nodes',
    sessions: '/web/cluster/:clusterId/sessions',
    recordings: '/web/cluster/:clusterId/recordings',
    databases: '/web/cluster/:clusterId/databases',
    desktops: '/web/cluster/:clusterId/desktops',
    desktop: '/web/cluster/:clusterId/desktops/:desktopName/:username',
    users: '/web/users',
    bots: '/web/bots',
    botsNew: '/web/bots/new/:type?',
    console: '/web/cluster/:clusterId/console',
    consoleNodes: '/web/cluster/:clusterId/console/nodes',
    consoleConnect: '/web/cluster/:clusterId/console/node/:serverId/:login',
    consoleSession: '/web/cluster/:clusterId/console/session/:sid',
    kubeExec: '/web/cluster/:clusterId/console/kube/exec/:kubeId/',
    kubeExecSession: '/web/cluster/:clusterId/console/kube/exec/:sid',
    player: '/web/cluster/:clusterId/session/:sid', // ?recordingType=ssh|desktop|k8s&durationMs=1234
    login: '/web/login',
    loginSuccess: '/web/msg/info/login_success',
    loginTerminalRedirect: '/web/msg/info/login_terminal',
    loginClose: '/web/msg/info/login_close',
    loginErrorLegacy: '/web/msg/error/login_failed',
    loginError: '/web/msg/error/login',
    loginErrorCallback: '/web/msg/error/login/callback',
    loginErrorUnauthorized: '/web/msg/error/login/auth',
    samlSloFailed: '/web/msg/error/slo',
    userInvite: '/web/invite/:tokenId',
    userInviteContinue: '/web/invite/:tokenId/continue',
    userReset: '/web/reset/:tokenId',
    userResetContinue: '/web/reset/:tokenId/continue',
    kubernetes: '/web/cluster/:clusterId/kubernetes',
    headlessSso: `/web/headless/:requestId`,
    integrations: '/web/integrations',
    integrationStatus: '/web/integrations/status/:type/:name',
    integrationEnroll: '/web/integrations/new/:type?',
    locks: '/web/locks',
    newLock: '/web/locks/new',
    requests: '/web/requests/:requestId?',

    downloadCenter: '/web/downloads',

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

    scp: '/v1/webapi/sites/:clusterId/nodes/:serverId/:login/scp?location=:location&filename=:filename&moderatedSessionId=:moderatedSessionId?&fileTransferRequestId=:fileTransferRequestId?',
    webRenewTokenPath: '/v1/webapi/sessions/web/renew',
    resetPasswordTokenPath: '/v1/webapi/users/password/token',
    webSessionPath: '/v1/webapi/sessions/web',
    userContextPath: '/v1/webapi/sites/:clusterId/context',
    userStatusPath: '/v1/webapi/user/status',
    passwordTokenPath: '/v1/webapi/users/password/token/:tokenId?',
    changeUserPasswordPath: '/v1/webapi/users/password',
    unifiedResourcesPath:
      '/v1/webapi/sites/:clusterId/resources?searchAsRoles=:searchAsRoles?&limit=:limit?&startKey=:startKey?&kinds=:kinds?&query=:query?&search=:search?&sort=:sort?&pinnedOnly=:pinnedOnly?&includedResourceMode=:includedResourceMode?',
    nodesPath:
      '/v1/webapi/sites/:clusterId/nodes?searchAsRoles=:searchAsRoles?&limit=:limit?&startKey=:startKey?&query=:query?&search=:search?&sort=:sort?',
    nodesPathNoParams: '/v1/webapi/sites/:clusterId/nodes',

    databaseServicesPath: `/v1/webapi/sites/:clusterId/databaseservices`,
    databaseIamPolicyPath: `/v1/webapi/sites/:clusterId/databases/:database/iam/policy`,
    databasePath: `/v1/webapi/sites/:clusterId/databases/:database`,
    databasesPath: `/v1/webapi/sites/:clusterId/databases?searchAsRoles=:searchAsRoles?&limit=:limit?&startKey=:startKey?&query=:query?&search=:search?&sort=:sort?`,

    desktopsPath: `/v1/webapi/sites/:clusterId/desktops?searchAsRoles=:searchAsRoles?&limit=:limit?&startKey=:startKey?&query=:query?&search=:search?&sort=:sort?`,
    desktopServicesPath: `/v1/webapi/sites/:clusterId/desktopservices?searchAsRoles=:searchAsRoles?&limit=:limit?&startKey=:startKey?&query=:query?&search=:search?&sort=:sort?`,
    desktopPath: `/v1/webapi/sites/:clusterId/desktops/:desktopName`,
    desktopWsAddr:
      'wss://:fqdn/v1/webapi/sites/:clusterId/desktops/:desktopName/connect/ws?username=:username',
    desktopPlaybackWsAddr:
      'wss://:fqdn/v1/webapi/sites/:clusterId/desktopplayback/:sid/ws',
    desktopIsActive: '/v1/webapi/sites/:clusterId/desktops/:desktopName/active',
    ttyWsAddr:
      'wss://:fqdn/v1/webapi/sites/:clusterId/connect/ws?params=:params&traceparent=:traceparent',
    ttyKubeExecWsAddr:
      'wss://:fqdn/v1/webapi/sites/:clusterId/kube/exec/ws?params=:params&traceparent=:traceparent',
    ttyPlaybackWsAddr:
      'wss://:fqdn/v1/webapi/sites/:clusterId/ttyplayback/:sid?access_token=:token', // TODO(zmb3): get token out of URL
    activeAndPendingSessionsPath: '/v1/webapi/sites/:clusterId/sessions',

    // TODO(zmb3): remove this when Assist is no longer using it
    sshPlaybackPrefix: '/v1/webapi/sites/:clusterId/sessions/:sid', // prefix because this is eventually concatenated with "/stream" or "/events"
    kubernetesPath:
      '/v1/webapi/sites/:clusterId/kubernetes?searchAsRoles=:searchAsRoles?&limit=:limit?&startKey=:startKey?&query=:query?&search=:search?&sort=:sort?',

    usersPath: '/v1/webapi/users',
    userWithUsernamePath: '/v1/webapi/users/:username',
    createPrivilegeTokenPath: '/v1/webapi/users/privilege/token',

    listRolesPath:
      '/v1/webapi/roles?startKey=:startKey?&search=:search?&limit=:limit?',
    rolePath: '/v1/webapi/roles/:name?',
    presetRolesPath: '/v1/webapi/presetroles',
    githubConnectorsPath: '/v1/webapi/github/:name?',
    trustedClustersPath: '/v1/webapi/trustedcluster/:name?',
    connectMyComputerLoginsPath: '/v1/webapi/connectmycomputer/logins',

    joinTokenPath: '/v1/webapi/token',
    joinTokenYamlPath: '/v1/webapi/tokens/yaml',
    joinTokensPath: '/v1/webapi/tokens',
    dbScriptPath: '/scripts/:token/install-database.sh',
    nodeScriptPath: '/scripts/:token/install-node.sh',
    appNodeScriptPath: '/scripts/:token/install-app.sh?name=:name&uri=:uri',

    discoveryConfigPath: '/v1/webapi/sites/:clusterId/discoveryconfig',

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

    captureUserEventPath: '/v1/webapi/capture',
    capturePreUserEventPath: '/v1/webapi/precapture',

    webapiPingPath: '/v1/webapi/ping',

    headlessLogin: '/v1/webapi/headless/:headless_authentication_id',

    integrationsPath: '/v1/webapi/sites/:clusterId/integrations/:name?',
    thumbprintPath: '/v1/webapi/thumbprint',

    awsConfigureIamScriptOidcIdpPath:
      '/v1/webapi/scripts/integrations/configure/awsoidc-idp.sh?integrationName=:integrationName&role=:roleName',
    awsConfigureIamScriptDeployServicePath:
      '/v1/webapi/scripts/integrations/configure/deployservice-iam.sh?integrationName=:integrationName&awsRegion=:region&role=:awsOidcRoleArn&taskRole=:taskRoleArn',
    awsConfigureIamScriptListDatabasesPath:
      '/v1/webapi/scripts/integrations/configure/listdatabases-iam.sh?awsRegion=:region&role=:iamRoleName',
    awsConfigureIamScriptEc2InstanceConnectPath:
      '/v1/webapi/scripts/integrations/configure/eice-iam.sh?awsRegion=:region&role=:iamRoleName',
    awsConfigureIamEksScriptPath:
      '/v1/webapi/scripts/integrations/configure/eks-iam.sh?awsRegion=:region&role=:iamRoleName',

    awsRdsDbDeployServicesPath:
      '/v1/webapi/sites/:clusterId/integrations/aws-oidc/:name/deploydatabaseservices',
    awsRdsDbRequiredVpcsPath:
      '/v1/webapi/sites/:clusterId/integrations/aws-oidc/:name/requireddatabasesvpcs',
    awsDatabaseVpcsPath:
      '/webapi/sites/:clusterId/integrations/aws-oidc/:name/databasevpcs',
    awsRdsDbListPath:
      '/v1/webapi/sites/:clusterId/integrations/aws-oidc/:name/databases',
    awsDeployTeleportServicePath:
      '/v1/webapi/sites/:clusterId/integrations/aws-oidc/:name/deployservice',
    awsSecurityGroupsListPath:
      '/v1/webapi/sites/:clusterId/integrations/aws-oidc/:name/securitygroups',
    awsSubnetListPath:
      '/v1/webapi/sites/:clusterId/integrations/aws-oidc/:name/subnets',

    awsAppAccessPath:
      '/v1/webapi/sites/:clusterId/integrations/aws-oidc/:name/aws-app-access',
    awsConfigureIamAppAccessPath:
      '/v1/webapi/scripts/integrations/configure/aws-app-access-iam.sh?role=:iamRoleName',

    awsConfigureIamEc2AutoDiscoverWithSsmPath:
      '/v1/webapi/scripts/integrations/configure/ec2-ssm-iam.sh?role=:iamRoleName&awsRegion=:region&ssmDocument=:ssmDocument&integrationName=:integrationName',

    eksClustersListPath:
      '/v1/webapi/sites/:clusterId/integrations/aws-oidc/:name/eksclusters',
    eksEnrollClustersPath:
      '/v1/webapi/sites/:clusterId/integrations/aws-oidc/:name/enrolleksclusters',

    ec2InstancesListPath:
      '/v1/webapi/sites/:clusterId/integrations/aws-oidc/:name/ec2',
    ec2InstanceConnectEndpointsListPath:
      '/v1/webapi/sites/:clusterId/integrations/aws-oidc/:name/ec2ice',
    // Returns a script that configures the required IAM permissions to enable the usage of EC2 Instance Connect Endpoint to access EC2 instances.
    ec2InstanceConnectDeployPath:
      '/v1/webapi/sites/:clusterId/integrations/aws-oidc/:name/deployec2ice',

    userGroupsListPath:
      '/v1/webapi/sites/:clusterId/user-groups?searchAsRoles=:searchAsRoles?&limit=:limit?&startKey=:startKey?&query=:query?&search=:search?&sort=:sort?',

    userPreferencesPath: '/v1/webapi/user/preferences',
    userClusterPreferencesPath: '/v1/webapi/user/preferences/:clusterId',

    // Assist needs some access request info to exist in OSS
    accessRequestPath: '/v1/enterprise/accessrequest/:requestId?',

    accessGraphFeatures: '/v1/enterprise/accessgraph/static/features.json',

    botsPath: '/v1/webapi/sites/:clusterId/machine-id/bot/:name?',
    botsTokenPath: '/v1/webapi/sites/:clusterId/machine-id/token',

    gcpWorkforceConfigurePath:
      '/webapi/scripts/integrations/configure/gcp-workforce-saml.sh?orgId=:orgId&poolName=:poolName&poolProviderName=:poolProviderName',

    notificationsPath:
      '/v1/webapi/sites/:clusterId/notifications?limit=:limit?&startKey=:startKey?',
    notificationLastSeenTimePath:
      '/v1/webapi/sites/:clusterId/lastseennotification',
    notificationStatePath: '/v1/webapi/sites/:clusterId/notificationstate',

    yaml: {
      parse: '/v1/webapi/yaml/parse/:kind',
      stringify: '/v1/webapi/yaml/stringify/:kind',
    },
  },

  playable_db_protocols: [],

  getNonExactRoutes() {
    // These routes will not be exact matched when deciding if it is a valid route
    // to redirect to when a user is unauthenticated.
    // This is useful for routes that can be infinitely nested, e.g. `
    // /web/accessgraph` and `/web/accessgraph/integrations/new`
    // (`/web/accessgraph/*` wouldn't work as it doesn't match `/web/accessgraph`)

    return [
      // at the moment, only `e` has an exempt route, which is merged in on `cfg.init()`
      ...this.nonExactRoutes,
    ];
  },

  getPlayableDatabaseProtocols() {
    return cfg.playable_db_protocols;
  },

  getUserClusterPreferencesUrl(clusterId: string) {
    return generatePath(cfg.api.userClusterPreferencesPath, {
      clusterId,
    });
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

  getMotd() {
    return cfg.auth.motd;
  },

  getLocalAuthFlag() {
    return cfg.auth.localAuthEnabled;
  },

  isPasswordlessEnabled() {
    return cfg.auth.allowPasswordless;
  },

  isMfaEnabled() {
    return cfg.auth.second_factor !== 'off';
  },

  isAdminActionMfaEnforced() {
    return cfg.auth.second_factor === 'webauthn';
  },

  getPrimaryAuthType(): PrimaryAuthType {
    if (cfg.auth.localConnectorName === 'passwordless') {
      return 'passwordless';
    }

    if (cfg.auth.authType === 'local') return 'local';

    return 'sso';
  },

  getDeviceTrustAuthorizeRoute(id: string, token: string) {
    return generatePath(cfg.routes.deviceTrustAuthorize, { id, token });
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

  getIntegrationStatusRoute(type: PluginKind, name: string) {
    return generatePath(cfg.routes.integrationStatus, { type, name });
  },

  getNodesRoute(clusterId: string) {
    return generatePath(cfg.routes.nodes, { clusterId });
  },

  getUnifiedResourcesRoute(clusterId: string) {
    return generatePath(cfg.routes.unifiedResources, { clusterId });
  },

  getDatabasesRoute(clusterId: string) {
    return generatePath(cfg.routes.databases, { clusterId });
  },

  getDesktopsRoute(clusterId: string) {
    return generatePath(cfg.routes.desktops, { clusterId });
  },

  getJoinTokensRoute() {
    return cfg.routes.joinTokens;
  },

  getJoinTokensUrl() {
    return cfg.api.joinTokensPath;
  },

  getJoinTokenUrl() {
    return cfg.api.joinTokenPath;
  },

  getJoinTokenYamlUrl() {
    return cfg.api.joinTokenYamlPath;
  },

  getNodeScriptUrl(token: string) {
    return cfg.baseUrl + generatePath(cfg.api.nodeScriptPath, { token });
  },

  getDeployServiceIamConfigureScriptUrl(
    p: UrlDeployServiceIamConfigureScriptParams
  ) {
    return (
      cfg.baseUrl +
      generatePath(cfg.api.awsConfigureIamScriptDeployServicePath, { ...p })
    );
  },

  getAwsOidcConfigureIdpScriptUrl(p: UrlAwsOidcConfigureIdp) {
    const path = cfg.api.awsConfigureIamScriptOidcIdpPath;
    return cfg.baseUrl + generatePath(path, { ...p });
  },

  getDbScriptUrl(token: string) {
    return cfg.baseUrl + generatePath(cfg.api.dbScriptPath, { token });
  },

  getAppNodeScriptUrl(token: string, name: string, uri: string) {
    return (
      cfg.baseUrl +
      generatePath(cfg.api.appNodeScriptPath, { token, name, uri })
    );
  },

  getUsersRoute() {
    return cfg.routes.users;
  },

  getBotsRoute() {
    return generatePath(cfg.routes.bots);
  },

  getBotsNewRoute(type?: string) {
    return generatePath(cfg.routes.botsNew, { type });
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

  getKubeExecConnectRoute(params: UrlKubeExecParams) {
    return generatePath(cfg.routes.kubeExec, { ...params });
  },

  getDesktopRoute({ clusterId, username, desktopName }) {
    return generatePath(cfg.routes.desktop, {
      clusterId,
      desktopName,
      username,
    });
  },

  getKubeExecSessionRoute(
    { clusterId, sid }: UrlParams,
    mode?: ParticipantMode
  ) {
    const basePath = generatePath(cfg.routes.kubeExecSession, {
      clusterId,
      sid,
    });
    if (mode) {
      return `${basePath}?mode=${mode}`;
    }
    return basePath;
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
    // TODO(zmb3): remove this when Assist is no longer using it
    return generatePath(cfg.api.sshPlaybackPrefix, { clusterId, sid });
  },

  getActiveAndPendingSessionsUrl({ clusterId }: UrlParams) {
    return generatePath(cfg.api.activeAndPendingSessionsPath, { clusterId });
  },

  getUnifiedResourcesUrl(clusterId: string, params: UrlResourcesParams) {
    return generateResourcePath(cfg.api.unifiedResourcesPath, {
      clusterId,
      ...params,
    });
  },

  getClusterNodesUrl(clusterId: string, params?: UrlResourcesParams) {
    return generateResourcePath(cfg.api.nodesPath, {
      clusterId,
      ...params,
    });
  },

  getClusterNodesUrlNoParams(clusterId: string) {
    return generatePath(cfg.api.nodesPathNoParams, { clusterId });
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

  getYamlParseUrl(kind: YamlSupportedResourceKind) {
    return generatePath(cfg.api.yaml.parse, { kind });
  },

  getYamlStringifyUrl(kind: YamlSupportedResourceKind) {
    return generatePath(cfg.api.yaml.stringify, { kind });
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

  getListRolesUrl(params?: UrlListRolesParams) {
    return generatePath(cfg.api.listRolesPath, {
      search: params?.search || undefined,
      startKey: params?.startKey || undefined,
      limit: params?.limit || undefined,
    });
  },

  getRoleUrl(name?: string) {
    return generatePath(cfg.api.rolePath, { name });
  },

  getDiscoveryConfigUrl(clusterId: string) {
    return generatePath(cfg.api.discoveryConfigPath, { clusterId });
  },

  getPresetRolesUrl() {
    return cfg.api.presetRolesPath;
  },

  getConnectMyComputerLoginsUrl() {
    return cfg.api.connectMyComputerLoginsPath;
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

  getAwsRdsDbRequiredVpcsUrl(integrationName: string) {
    const clusterId = cfg.proxyCluster;

    return generatePath(cfg.api.awsRdsDbRequiredVpcsPath, {
      clusterId,
      name: integrationName,
    });
  },

  getAwsDatabaseVpcsUrl(integrationName: string, clusterId: string) {
    return generatePath(cfg.api.awsDatabaseVpcsPath, {
      clusterId,
      name: integrationName,
    });
  },

  getAwsRdsDbsDeployServicesUrl(integrationName: string) {
    const clusterId = cfg.proxyCluster;

    return generatePath(cfg.api.awsRdsDbDeployServicesPath, {
      clusterId,
      name: integrationName,
    });
  },

  getAwsDeployTeleportServiceUrl(integrationName: string) {
    const clusterId = cfg.proxyCluster;

    return generatePath(cfg.api.awsDeployTeleportServicePath, {
      clusterId,
      name: integrationName,
    });
  },

  getAwsAppAccessUrl(integrationName: string) {
    const clusterId = cfg.proxyCluster;

    return generatePath(cfg.api.awsAppAccessPath, {
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

  getAccessRequestUrl(requestId?: string) {
    return generatePath(cfg.api.accessRequestPath, { requestId });
  },

  getAccessRequestRoute(requestId?: string) {
    return generatePath(cfg.routes.requests, { requestId });
  },

  getAccessGraphFeaturesUrl() {
    return cfg.api.accessGraphFeatures;
  },

  getEnrollEksClusterUrl(integrationName: string): string {
    const clusterId = cfg.proxyCluster;

    return generatePath(cfg.api.eksEnrollClustersPath, {
      clusterId,
      name: integrationName,
    });
  },

  getListEKSClustersUrl(integrationName: string): string {
    const clusterId = cfg.proxyCluster;

    return generatePath(cfg.api.eksClustersListPath, {
      clusterId,
      name: integrationName,
    });
  },

  getListEc2InstancesUrl(integrationName: string) {
    const clusterId = cfg.proxyCluster;

    return generatePath(cfg.api.ec2InstancesListPath, {
      clusterId,
      name: integrationName,
    });
  },

  getListEc2InstanceConnectEndpointsUrl(integrationName: string) {
    const clusterId = cfg.proxyCluster;

    return generatePath(cfg.api.ec2InstanceConnectEndpointsListPath, {
      clusterId,
      name: integrationName,
    });
  },

  getDeployEc2InstanceConnectEndpointUrl(integrationName: string) {
    const clusterId = cfg.proxyCluster;

    return generatePath(cfg.api.ec2InstanceConnectDeployPath, {
      clusterId,
      name: integrationName,
    });
  },

  getListSecurityGroupsUrl(integrationName: string) {
    const clusterId = cfg.proxyCluster;

    return generatePath(cfg.api.awsSecurityGroupsListPath, {
      clusterId,
      name: integrationName,
    });
  },

  getAwsSubnetListUrl(integrationName: string, clusterId: string) {
    return generatePath(cfg.api.awsSubnetListPath, {
      clusterId,
      name: integrationName,
    });
  },

  getEc2InstanceConnectIAMConfigureScriptUrl(
    params: UrlAwsConfigureIamScriptParams
  ) {
    return (
      cfg.baseUrl +
      generatePath(cfg.api.awsConfigureIamScriptEc2InstanceConnectPath, {
        ...params,
      })
    );
  },

  getEksIamConfigureScriptUrl(params: UrlAwsConfigureIamScriptParams) {
    return (
      cfg.baseUrl +
      generatePath(cfg.api.awsConfigureIamEksScriptPath, {
        ...params,
      })
    );
  },

  getAwsIamConfigureScriptAppAccessUrl(
    params: Omit<UrlAwsConfigureIamScriptParams, 'region'>
  ) {
    return (
      cfg.baseUrl +
      generatePath(cfg.api.awsConfigureIamAppAccessPath, {
        ...params,
      })
    );
  },

  getAwsIamConfigureScriptEc2AutoDiscoverWithSsmUrl(
    params: UrlAwsConfigureIamEc2AutoDiscoverWithSsmScriptParams
  ) {
    return (
      cfg.baseUrl +
      generatePath(cfg.api.awsConfigureIamEc2AutoDiscoverWithSsmPath, {
        ...params,
      })
    );
  },

  getAwsConfigureIamScriptListDatabasesUrl(
    params: UrlAwsConfigureIamScriptParams
  ) {
    return (
      cfg.baseUrl +
      generatePath(cfg.api.awsConfigureIamScriptListDatabasesPath, {
        ...params,
      })
    );
  },

  getBotTokenUrl() {
    const clusterId = cfg.proxyCluster;
    return generatePath(cfg.api.botsTokenPath, { clusterId });
  },

  getBotsUrl() {
    const clusterId = cfg.proxyCluster;
    return generatePath(cfg.api.botsPath, { clusterId });
  },

  getBotUrlWithName(name: string) {
    const clusterId = cfg.proxyCluster;
    return generatePath(cfg.api.botsPath, { clusterId, name });
  },

  getGcpWorkforceConfigScriptUrl(p: UrlGcpWorkforceConfigParam) {
    return (
      cfg.baseUrl + generatePath(cfg.api.gcpWorkforceConfigurePath, { ...p })
    );
  },

  getNotificationsUrl(params: UrlNotificationParams) {
    return generatePath(cfg.api.notificationsPath, { ...params });
  },

  getNotificationLastSeenUrl(clusterId: string) {
    return generatePath(cfg.api.notificationLastSeenTimePath, { clusterId });
  },

  getNotificationStateUrl(clusterId: string) {
    return generatePath(cfg.api.notificationStatePath, { clusterId });
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

export interface UrlKubeExecParams {
  clusterId: string;
  kubeId: string;
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
  durationMs?: number;
}

// /web/cluster/:clusterId/desktops/:desktopName/:username
export interface UrlDesktopParams {
  username?: string;
  desktopName?: string;
  clusterId: string;
}

export interface UrlListRolesParams {
  search?: string;
  limit?: number;
  startKey?: string;
}

export interface UrlResourcesParams {
  query?: string;
  search?: string;
  sort?: SortType;
  limit?: number;
  startKey?: string;
  searchAsRoles?: 'yes' | '';
  pinnedOnly?: boolean;
  includedResourceMode?: IncludedResourceMode;
  // TODO(bl-nero): Remove this once filters are expressed as advanced search.
  kinds?: string[];
}

export interface UrlDeployServiceIamConfigureScriptParams {
  integrationName: string;
  region: Regions;
  awsOidcRoleArn: string;
  taskRoleArn: string;
}

export interface UrlAwsOidcConfigureIdp {
  integrationName: string;
  roleName: string;
  s3Bucket?: string;
  s3Prefix?: string;
}

export interface UrlAwsConfigureIamScriptParams {
  region: Regions;
  iamRoleName: string;
}

export interface UrlAwsConfigureIamEc2AutoDiscoverWithSsmScriptParams {
  region: Regions;
  iamRoleName: string;
  ssmDocument: string;
  integrationName: string;
}

export interface UrlGcpWorkforceConfigParam {
  orgId: string;
  poolName: string;
  poolProviderName: string;
}

export interface UrlNotificationParams {
  clusterId: string;
  limit?: number;
  startKey?: string;
}

export default cfg;

export type TeleportEdition = 'ent' | 'community' | 'oss';
