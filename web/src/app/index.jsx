var React = require('react');
var { Router, Route, Redirect, IndexRoute } = require('react-router');
var render = require('react-dom').render;

// route component modules
var App = require('./components/app.jsx');
var Login = require('./components/login.jsx');
var Nodes = require('./components/nodes/main.jsx');
var Sessions = require('./components/sessions/main.jsx');
var NewUser = require('./components/newUser.jsx');
var auth = require('./auth');
var session = require('./session');

// init session
session.init();

function requireAuth(nextState, replace, cb) {
  auth.ensureUser()
    .done(()=> cb())
    .fail(()=>{
      replace({redirectTo: nextState.location.pathname }, '/web/login' );
      cb();
    });
}

function handleLogout(nextState, replace, cb){
  auth.logout();
  // going back will hit requireAuth handler which will redirect it to the login page
  session.getHistory().goBack();
}

render((
  <Router history={session.getHistory()}>
    <Route path="/web/login" component={Login}/>
    <Route path="/web/logout" onEnter={handleLogout}/>
    <Route path="/web/newuser" component={NewUser}/>
    <Route path="/web" component={App}>
      <Route path="nodes" component={Nodes}/>
      <Route path="sessions" component={Sessions}/>
    </Route>
  </Router>
), document.getElementById("app"));
