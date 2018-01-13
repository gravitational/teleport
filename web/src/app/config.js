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

import { formatPattern } from 'app/lib/patternUtils';
import $ from 'jQuery';
import { isTestEnv } from './services/utils'

const baseUrl = isTestEnv() ? 'localhost' : window.location.origin;

const cfg = {

  baseUrl,

  helpUrl: 'https://gravitational.com/teleport/docs/quickstart/',
  
  maxSessionLoadSize: 50,

  displayDateFormat: 'MM/DD/YYYY HH:mm:ss',

  auth: {        
  },

  canJoinSessions: true,

  routes: {
    app: '/web',
    login: '/web/login',    
    nodes: '/web/nodes',
    currentSession: '/web/cluster/:siteId/sessions/:sid',
    sessions: '/web/sessions',
    newUser: '/web/newuser/:inviteToken',    
    error: '/web/msg/error(/:type)',
    info: '/web/msg/info(/:type)',
    pageNotFound: '/web/notfound',
    terminal: '/web/cluster/:siteId/node/:serverId/:login(/:sid)',
    player: '/web/player/cluster/:siteId/sid/:sid',
    webApi: '/v1/webapi/*',                
    settingsBase: '/web/settings',        
    settingsAccount: '/web/settings/account',        
  },

  api: {    
    ssoOidc: '/v1/webapi/oidc/login/web?redirect_url=:redirect&connector_id=:providerName',    
    ssoSaml: '/v1/webapi/saml/sso?redirect_url=:redirect&connector_id=:providerName',        
    renewTokenPath:'/v1/webapi/sessions/renew',
    sessionPath: '/v1/webapi/sessions',
    userContextPath: '/v1/webapi/user/context',
    userStatusPath: '/v1/webapi/user/status',    
    invitePath: '/v1/webapi/users/invites/:inviteToken',        
    createUserPath: '/v1/webapi/users',
    changeUserPasswordPath: '/v1/webapi/users/password',
    u2fCreateUserChallengePath: '/v1/webapi/u2f/signuptokens/:inviteToken',
    u2fCreateUserPath: '/v1/webapi/u2f/users',
    u2fSessionChallengePath: '/v1/webapi/u2f/signrequest',
    u2fChangePassChallengePath: '/v1/webapi/u2f/password/changerequest',
    u2fChangePassPath: '/v1/webapi/u2f/password',
    u2fSessionPath: '/v1/webapi/u2f/sessions',
    sitesBasePath: '/v1/webapi/sites',
    sitePath: '/v1/webapi/sites/:siteId',  
    nodesPath: '/v1/webapi/sites/:siteId/nodes',
    siteSessionPath: '/v1/webapi/sites/:siteId/sessions',
    sessionEventsPath: '/v1/webapi/sites/:siteId/sessions/:sid/events',
    siteEventSessionFilterPath: `/v1/webapi/sites/:siteId/sessions`,
    siteEventsFilterPath: `/v1/webapi/sites/:siteId/events?event=session.start&event=session.end&from=:start&to=:end`,
    ttyWsAddr: ':fqdm/v1/webapi/sites/:cluster/connect?access_token=:token&params=:params',
    ttyEventWsAddr: ':fqdm/v1/webapi/sites/:cluster/sessions/:sid/events/stream?access_token=:token',      
    ttyResizeUrl: '/v1/webapi/sites/:cluster/sessions/:sid',

    getSiteUrl(siteId) {              
      return formatPattern(cfg.api.sitePath, { siteId });  
    },

    getSiteNodesUrl(siteId='-current-') {
      return formatPattern(cfg.api.nodesPath, { siteId });
    },

    getSiteSessionUrl(siteId='-current-') {
      return formatPattern(cfg.api.siteSessionPath, { siteId });  
    },

    getSsoUrl(providerUrl, providerName, redirect) {                  
      return cfg.baseUrl + formatPattern(providerUrl, { redirect, providerName });              
    },
    
    getSiteEventsFilterUrl({start, end, siteId}){
      return formatPattern(cfg.api.siteEventsFilterPath, {start, end, siteId});
    },

    getSessionEventsUrl({sid, siteId}){
      return formatPattern(cfg.api.sessionEventsPath, {sid, siteId});
    },

    getFetchSessionsUrl(siteId){      
      return formatPattern(cfg.api.siteEventSessionFilterPath, {siteId});
    },

    getFetchSessionUrl({sid, siteId}){
      return formatPattern(cfg.api.siteSessionPath+'/:sid', {sid, siteId});
    },

    getInviteUrl(inviteToken){
      return formatPattern(cfg.api.invitePath, {inviteToken});
    },
    
    getU2fCreateUserChallengeUrl(inviteToken){
      return formatPattern(cfg.api.u2fCreateUserChallengePath, {inviteToken});
    }
  },

  getPlayerUrl({siteId, sid}) {
    return formatPattern(cfg.routes.player, { siteId, sid });  
  },

  getTerminalLoginUrl({siteId, serverId, login, sid}) {
    if (!sid) {
      let url = this.stripOptionalParams(cfg.routes.terminal);
      return formatPattern(url, {siteId, serverId, login });      
    }

    return formatPattern(cfg.routes.terminal, { siteId, serverId, login, sid });  
  },
  
  getCurrentSessionRouteUrl({sid, siteId}){
    return formatPattern(cfg.routes.currentSession, {sid, siteId});
  },

  getAuthProviders() {            
    return cfg.auth && cfg.auth.providers ? cfg.auth.providers : [];              
  },
    
  getAuth2faType() {
    return cfg.auth ? cfg.auth.second_factor : null; 
  },

  getU2fAppId(){    
    return cfg.auth && cfg.auth.u2f ? cfg.auth.u2f.app_id : null;    
  },

  getWsHostName(){
    const prefix = location.protocol === 'https:' ? 'wss://' : 'ws://';
    const hostport = location.hostname+(location.port ? ':'+location.port: '');
    return `${prefix}${hostport}`;
  },
  
  init(config = {}) {    
    $.extend(true, this, config);
  },
    
  stripOptionalParams(pattern) {
    return pattern.replace(/\(.*\)/, '');
  } 
}

export default cfg;
