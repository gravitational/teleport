let {formatPattern} = require('app/common/patternUtils');

let cfg = {

  baseUrl: window.location.origin,

  api: {
    nodesPath: '/v1/webapi/sites/-current-/nodes',
    sessionPath: '/v1/webapi/sessions',
    invitePath: '/v1/webapi/users/invites/:inviteToken',
    createUserPath: '/v1/webapi/users',
    getInviteUrl: (inviteToken) => {
      return formatPattern(cfg.api.invitePath, {inviteToken});
    }
  },

  routes: {
    app: '/web',
    logout: '/web/logout',
    login: '/web/login',
    nodes: '/web/nodes',
    newUser: '/web/newuser/:inviteToken',
    sessions: '/web/sessions'
  }

}

export default cfg;
