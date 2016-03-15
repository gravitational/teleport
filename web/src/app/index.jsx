var React = require('react');
var render = require('react-dom').render;
var { Router, Route, Redirect, IndexRoute, browserHistory } = require('react-router');
var { App, Login, Nodes, Sessions, NewUser, CurrentSessionHost, NotFound } = require('./components');
var {ensureUser} = require('./modules/user/actions');
var auth = require('./auth');
var session = require('./session');
var cfg = require('./config');

require('./modules');

// init session
session.init();

function handleLogout(nextState, replace, cb){
  auth.logout();
}

render((
  <Router history={session.getHistory()}>
    <Route path={cfg.routes.login} component={Login}/>
    <Route path={cfg.routes.logout} onEnter={handleLogout}/>
    <Route path={cfg.routes.newUser} component={NewUser}/>
    <Redirect from={cfg.routes.app} to={cfg.routes.nodes}/>
    <Route path={cfg.routes.app} component={App} onEnter={ensureUser} >
      <Route path={cfg.routes.nodes} component={Nodes}/>
      <Route path={cfg.routes.activeSession} components={{CurrentSessionHost: CurrentSessionHost}}/>
      <Route path={cfg.routes.sessions} component={Sessions}/>
    </Route>
    <Route path="*" component={NotFound} />
  </Router>
), document.getElementById("app"));
