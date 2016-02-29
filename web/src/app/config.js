let {formatPattern} = require('app/common/patternUtils');

let cfg = {

  baseUrl: window.location.origin,

  api: {
    renewTokenPath:'/v1/webapi/sessions/renew',
    nodesPath: '/v1/webapi/sites/-current-/nodes',
    sessionPath: '/v1/webapi/sessions',
    invitePath: '/v1/webapi/users/invites/:inviteToken',
    createUserPath: '/v1/webapi/users',
    getInviteUrl: (inviteToken) => {
      return formatPattern(cfg.api.invitePath, {inviteToken});
    },

    getEventStreamerConnStr: (token, sid) => {
      var hostname = getWsHostName();
      return `${hostname}/v1/webapi/sites/-current-/sessions/${sid}/events/stream?access_token=${token}`;
    },

    getSessionConnStr: (token, params) => {
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
    activeSession: '/web/active-session/:sid',
    newUser: '/web/newuser/:inviteToken',
    sessions: '/web/sessions'
  }

}

export default cfg;

function getWsHostName(){
  var prefix = location.protocol == "https:"?"wss://":"ws://";
  var hostport = location.hostname+(location.port ? ':'+location.port: '');
  return `${prefix}${hostport}`;
}
