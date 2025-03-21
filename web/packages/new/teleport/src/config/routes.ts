export interface RouteRecord {
  [key: string]: string | RouteRecord;
}

export const routePaths = {
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
  manageCluster: '/web/clusters/:clusterId/manage',

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
  dbConnect: '/web/cluster/:clusterId/console/db/connect/:serviceName',
  dbSession: '/web/cluster/:clusterId/console/db/session/:sid',
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
  integrationStatus: '/web/integrations/status/:type/:name/:subPage?',
  integrationTasks: '/web/integrations/status/:type/:name/tasks',
  integrationStatusResources:
    '/web/integrations/status/:type/:name/resources/:resourceKind',
  integrationEnroll: '/web/integrations/new/:type?/:subPage?',
  locks: '/web/locks',
  newLock: '/web/locks/new',
  requests: '/web/requests/:requestId?',

  downloadCenter: '/web/downloads',

  // sso routes
  ssoConnector: {
    /**
     * create is the dedicated page for creating a new auth connector.
     */
    create: '/web/sso/new/:connectorType(github|oidc|saml)',
    edit: '/web/sso/edit/:connectorType(github|oidc|saml)/:connectorName?',
  },

  // whitelist sso handlers
  oidcHandler: '/v1/webapi/oidc/*',
  samlHandler: '/v1/webapi/saml/*',
  githubHandler: '/v1/webapi/github/*',

  // Access Graph is part of enterprise, but we need to generate links in the audit log,
  // which is in OSS.
  accessGraph: {
    base: '/web/accessgraph',
    crownJewelAccessPath: '/web/accessgraph/crownjewels/access/:id',
  },

  /** samlIdpSso is an exact path of the service provider initiated SAML SSO endpoint. */
  samlIdpSso: '/enterprise/saml-idp/sso',
};
