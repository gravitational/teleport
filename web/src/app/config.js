let {formatPattern} = require('app/common/patternUtils');

let cfg = {

  baseUrl: window.location.origin,

  helpUrl: 'https://github.com/gravitational/teleport/blob/master/README.md',

  api: {
    renewTokenPath:'/v1/webapi/sessions/renew',
    nodesPath: '/v1/webapi/sites/-current-/nodes',
    sessionPath: '/v1/webapi/sessions',
    siteSessionPath: '/v1/webapi/sites/-current-/sessions/:sid',
    invitePath: '/v1/webapi/users/invites/:inviteToken',
    createUserPath: '/v1/webapi/users',
    sessionChunk: '/v1/webapi/sites/-current-/sessions/:sid/chunks?start=:start&end=:end',
    sessionChunkCountPath: '/v1/webapi/sites/-current-/sessions/:sid/chunkscount',

    getFetchSessionChunkUrl: ({sid, start, end})=>{
      return formatPattern(cfg.api.sessionChunk, {sid, start, end});
    },

    getFetchSessionLengthUrl: (sid)=>{
      return formatPattern(cfg.api.sessionChunkCountPath, {sid});
    },

    getFetchSessionsUrl: (/*start, end*/)=>{
      var params = {
        start: new Date().toISOString(),
        order: -1
      };

      var json = JSON.stringify(params);
      var jsonEncoded = window.encodeURI(json);

      return `/v1/webapi/sites/-current-/events/sessions?filter=${jsonEncoded}`;
    },

    getFetchSessionUrl: (sid)=>{
      return formatPattern(cfg.api.siteSessionPath, {sid});
    },

    getTerminalSessionUrl: (sid)=> {
      return formatPattern(cfg.api.siteSessionPath, {sid});
    },

    getInviteUrl: (inviteToken) => {
      return formatPattern(cfg.api.invitePath, {inviteToken});
    },

    getEventStreamConnStr: (token, sid) => {
      var hostname = getWsHostName();
      return `${hostname}/v1/webapi/sites/-current-/sessions/${sid}/events/stream?access_token=${token}`;
    },

    getTtyConnStr: ({token, serverId, login, sid, rows, cols}) => {
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
