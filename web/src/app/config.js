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

let {formatPattern} = require('app/common/patternUtils');

let cfg = {

  baseUrl: window.location.origin,

  helpUrl: 'http://gravitational.com/teleport/docs/quickstart/',

  maxSessionLoadSize: 50,

  displayDateFormat: 'l LTS Z',

  routes: {
    app: '/web',
    logout: '/web/logout',
    login: '/web/login',
    nodes: '/web/nodes',
    activeSession: '/web/sessions/:sid',
    newUser: '/web/newuser/:inviteToken',
    sessions: '/web/sessions',
    pageNotFound: '/web/notfound'
  },

  api: {
    sso: '/v1/webapi/oidc/login?redirect_url=:redirect',
    renewTokenPath:'/v1/webapi/sessions/renew',
    nodesPath: '/v1/webapi/sites/-current-/nodes',
    sessionPath: '/v1/webapi/sessions',
    siteSessionPath: '/v1/webapi/sites/-current-/sessions',
    invitePath: '/v1/webapi/users/invites/:inviteToken',
    createUserPath: '/v1/webapi/users',
    sessionChunk: '/v1/webapi/sites/-current-/sessions/:sid/chunks?start=:start&end=:end',
    sessionChunkCountPath: '/v1/webapi/sites/-current-/sessions/:sid/chunkscount',
    siteEventSessionFilterPath: `/v1/webapi/sites/-current-/events/sessions?filter=:filter`,

    getSsoUrl(redirect){
      return cfg.baseUrl + formatPattern(cfg.api.sso, {redirect});
    },

    getFetchSessionChunkUrl({sid, start, end}){
      return formatPattern(cfg.api.sessionChunk, {sid, start, end});
    },

    getFetchSessionLengthUrl(sid){
      return formatPattern(cfg.api.sessionChunkCountPath, {sid});
    },

    getFetchSessionsUrl(args){
      var filter = JSON.stringify(args);
      return formatPattern(cfg.api.siteEventSessionFilterPath, {filter});
    },

    getFetchSessionUrl(sid){
      return formatPattern(cfg.api.siteSessionPath+'/:sid', {sid});
    },

    getTerminalSessionUrl(sid){
      return formatPattern(cfg.api.siteSessionPath+'/:sid', {sid});
    },

    getInviteUrl(inviteToken){
      return formatPattern(cfg.api.invitePath, {inviteToken});
    },

    getEventStreamConnStr(token, sid){
      var hostname = getWsHostName();
      return `${hostname}/v1/webapi/sites/-current-/sessions/${sid}/events/stream?access_token=${token}`;
    },

    getTtyConnStr({token, serverId, login, sid, rows, cols}){
      var params = {
        server_id: serverId,
        login,
        sid,
        term: {
          h: rows,
          w: cols
        }
      }

      var json = JSON.stringify(params);
      var jsonEncoded = window.encodeURI(json);
      var hostname = getWsHostName();
      return `${hostname}/v1/webapi/sites/-current-/connect?access_token=${token}&params=${jsonEncoded}`;
    }
  },

  getFullUrl(url){
    return cfg.baseUrl + url;
  },

  getActiveSessionRouteUrl(sid){
    return formatPattern(cfg.routes.activeSession, {sid});
  }
}

export default cfg;

function getWsHostName(){
  var prefix = location.protocol == "https:"?"wss://":"ws://";
  var hostport = location.hostname+(location.port ? ':'+location.port: '');
  return `${prefix}${hostport}`;
}
