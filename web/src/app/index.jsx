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

var React = require('react');
var render = require('react-dom').render;
var { Router, Route, Redirect } = require('react-router');
var { App, Login, Nodes, Sessions, NewUser, CurrentSessionHost, MessagePage, NotFound } = require('./components');
var {ensureUser} = require('./modules/user/actions');
var auth = require('./services/auth');
var session = require('./services/session');
var cfg = require('./config');

require('./modules');

// init session
session.init();

cfg.init(window.GRV_CONFIG);

render((
  <Router history={session.getHistory()}>
    <Route path={cfg.routes.msgs} component={MessagePage}/>
    <Route path={cfg.routes.login} component={Login}/>
    <Route path={cfg.routes.logout} onEnter={auth.logout}/>
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
