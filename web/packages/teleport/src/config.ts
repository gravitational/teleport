/*
Copyright 2015 Gravitational, Inc.

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
import { merge } from 'lodash';

const cfg = {
  baseUrl: window.location.origin,

  auth: {
    localAuthEnabled: true,
    providers: [],
    second_factor: 'off',
  },

  canJoinSessions: true,

  clusterName: 'localhost',

  proxyCluster: 'localhost',

  loc: {
    dateTimeFormat: 'YYYY-MM-DD HH:mm:ss',
    dateFormat: 'YYYY-MM-DD',
  },

  routes: {
    app: '/web',
    account: '/web/account',
    cluster: '/web/cluster/:clusterId',
    clusterAccount: '/web/cluster/:clusterId/account',
    clusterAudit: '/web/cluster/:clusterId/audit',
    clusterAuditEvents: '/web/cluster/:clusterId/audit/events',
    clusterAuditSessions: '/web/cluster/:clusterId/audit/sessions',
    clusterNodes: '/web/cluster/:clusterId/nodes',
    clusterSessions: '/web/cluster/:clusterId/sessions',
    console: '/web/cluster/:clusterId/console',
    consoleNodes: '/web/cluster/:clusterId/console/nodes',
    consoleConnect: '/web/cluster/:clusterId/console/node/:serverId/:login',
    consoleSession: '/web/cluster/:clusterId/console/session/:sid',
    player: '/web/cluster/:clusterId/session/:sid',
    sessionAuditPlayer: '/web/cluster/:clusterId/session/:sid/player',
    sessionAuditCmds: '/web/cluster/:clusterId/session/:sid/commands',
    error: '/web/msg/error/:type?',
    login: '/web/login',
    loginFailed: '/web/msg/error/login_failed',
    loginSuccess: '/web/msg/info/login_success',
    userInvite: '/web/invite/:tokenId',
    userReset: '/web/reset/:tokenId',
    // whitelist sso handlers
    oidcHandler: '/v1/webapi/oidc/*',
    samlHandler: '/v1/webapi/saml/*',
    githubHandler: '/v1/webapi/github/*',
  },

  api: {
    clustersPath: '/v1/webapi/sites',
    clusterEventsPath: `/v1/webapi/sites/:clusterId/events/search?from=:start?&to=:end?&limit=:limit?`,
    scp:
      '/v1/webapi/sites/:clusterId/nodes/:serverId/:login/scp?location=:location&filename=:filename',
    renewTokenPath: '/v1/webapi/sessions/renew',
    sessionPath: '/v1/webapi/sessions',
    userContextPath: '/v1/webapi/sites/:clusterId/context',
    userStatusPath: '/v1/webapi/user/status',
    passwordTokenPath: '/v1/webapi/users/password/token/:tokenId?',
    userTokenInviteDonePath: '/v1/webapi/users',
    changeUserPasswordPath: '/v1/webapi/users/password',
    u2fCreateUserChallengePath: '/v1/webapi/u2f/signuptokens/:tokenId',
    u2fSessionChallengePath: '/v1/webapi/u2f/signrequest',
    u2fChangePassChallengePath: '/v1/webapi/u2f/password/changerequest',
    u2fSessionPath: '/v1/webapi/u2f/sessions',
    nodesPath: '/v1/webapi/sites/:clusterId/nodes',
    siteSessionPath: '/v1/webapi/sites/:siteId/sessions',
    sessionEventsPath: '/v1/webapi/sites/:siteId/sessions/:sid/events',
    siteEventSessionFilterPath: `/v1/webapi/sites/:siteId/sessions`,
    siteEventsFilterPath: `/v1/webapi/sites/:siteId/events?event=session.start&event=session.end&from=:start&to=:end`,
    ttyWsAddr:
      'wss://:fqdm/v1/webapi/sites/:clusterId/connect?access_token=:token&params=:params',
    terminalSessionPath: '/v1/webapi/sites/:clusterId/sessions/:sid?',
  },

  getClusterEventsUrl(params: UrlClusterEventsParams) {
    const clusterId = cfg.clusterName;
    return generatePath(cfg.api.clusterEventsPath, {
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

  getLocalAuthFlag() {
    return cfg.auth.localAuthEnabled;
  },

  getSsoUrl(providerUrl, providerName, redirect) {
    return cfg.baseUrl + generatePath(providerUrl, { redirect, providerName });
  },

  getDefaultRoute() {
    const clusterId = cfg.proxyCluster;
    return generatePath(cfg.routes.cluster, { clusterId });
  },

  getAuditRoute() {
    const clusterId = cfg.clusterName;
    return generatePath(cfg.routes.clusterAudit, { clusterId });
  },

  getDashboardRoute() {
    return cfg.routes.app;
  },

  getNodesRoute() {
    const clusterId = cfg.clusterName;
    return generatePath(cfg.routes.clusterNodes, { clusterId });
  },

  getSessionsRoute() {
    const clusterId = cfg.clusterName;
    return generatePath(cfg.routes.clusterSessions, { clusterId });
  },

  getConsoleNodesRoute(clusterId: string) {
    clusterId = clusterId || cfg.clusterName;
    return generatePath(cfg.routes.consoleNodes, {
      clusterId,
    });
  },

  getSshConnectRoute({ clusterId, login, serverId }: UrlParams) {
    clusterId = clusterId || cfg.clusterName;
    return generatePath(cfg.routes.consoleConnect, {
      clusterId,
      serverId,
      login,
    });
  },

  getSshSessionRoute({ clusterId, sid }: UrlParams) {
    clusterId = clusterId || cfg.clusterName;
    return generatePath(cfg.routes.consoleSession, { clusterId, sid });
  },

  getAuditEventsRoute(clusterId?: string) {
    clusterId = clusterId || cfg.clusterName;
    return generatePath(cfg.routes.clusterAuditEvents, { clusterId });
  },

  getAuditSessionsRoute(clusterId?: string) {
    clusterId = clusterId || cfg.clusterName;
    return generatePath(cfg.routes.clusterAuditSessions, { clusterId });
  },

  getPasswordTokenUrl(tokenId) {
    return generatePath(cfg.api.passwordTokenPath, { tokenId });
  },

  getClusterRoute(clusterId: string) {
    clusterId = clusterId || cfg.clusterName;
    return generatePath(cfg.routes.cluster, { clusterId });
  },

  getConsoleRoute(clusterId: string) {
    clusterId = clusterId || cfg.clusterName;
    return generatePath(cfg.routes.console, { clusterId });
  },

  getPlayerRoute({ clusterId, sid }: UrlParams) {
    clusterId = clusterId || cfg.clusterName;
    return generatePath(cfg.routes.player, { clusterId, sid });
  },

  getSessionAuditPlayerRoute({ clusterId, sid }: UrlParams) {
    clusterId = clusterId || cfg.clusterName;
    return generatePath(cfg.routes.sessionAuditPlayer, { clusterId, sid });
  },

  getSessionAuditCmdsRoute({ clusterId, sid }: UrlParams) {
    clusterId = clusterId || cfg.clusterName;
    return generatePath(cfg.routes.sessionAuditCmds, { clusterId, sid });
  },

  getUserUrl(clusterId?: string) {
    clusterId = clusterId || cfg.clusterName;
    return generatePath(cfg.api.userContextPath, { clusterId });
  },

  getTerminalSessionUrl({ clusterId, sid }: UrlParams) {
    clusterId = clusterId || cfg.clusterName;
    return generatePath(cfg.api.terminalSessionPath, { clusterId, sid });
  },

  getClusterNodesUrl(clusterId: string) {
    clusterId = clusterId || cfg.clusterName;
    return generatePath(cfg.api.nodesPath, { clusterId });
  },

  getU2fCreateUserChallengeUrl(tokenId: string) {
    return generatePath(cfg.api.u2fCreateUserChallengePath, { tokenId });
  },

  getScpUrl(params: UrlScpParams) {
    return generatePath(cfg.api.scp, {
      ...params,
    });
  },

  setClusterId(clusterId: string) {
    cfg.clusterName = clusterId || cfg.proxyCluster;
  },

  init(newConfig = {}) {
    merge(this, newConfig);
  },
};

export interface UrlParams {
  sid?: string;
  clusterId?: string;
  login?: string;
  serverId?: string;
}

export interface UrlScpParams {
  clusterId: string;
  serverId: string;
  login: string;
  location: string;
  filename: string;
}

export interface UrlSshParams {
  login?: string;
  serverId?: string;
  sid?: string;
  clusterId?: string;
}

export interface UrlClusterEventsParams {
  start: string;
  end: string;
  limit?: number;
}

export default cfg;
