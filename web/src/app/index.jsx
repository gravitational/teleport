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

import React from 'react';
import { render } from 'react-dom';
import { Router, Route, Redirect } from 'react-router';
import { Provider } from 'nuclear-js-react-addons';
import history from './services/history';
import AppContainer from './components/app.jsx';
import LoginContainer from './components/user/login.jsx';
import InviteUser from './components/user/invite.jsx';
import Nodes from './components/nodes/main.jsx';
import Sessions from './components/sessions/main.jsx';
import TerminalHost from './components/terminal/terminalHost.jsx';
import PlayerHost from './components/player/playerHost.jsx';
import  * as Message from './components/msgPage.jsx';
import { ensureUser, initLogin } from './flux/user/actions';
import { initApp } from './flux/app/actions';
import cfg from './config';
import reactor from './reactor';
import DocumentTitle from './components/documentTitle';
import './flux';

history.init();

cfg.init(window.GRV_CONFIG);

render((
  <Provider reactor={reactor}>        
    <Router history={history.original()}>      
      <Route component={DocumentTitle}>
        <Route path={cfg.routes.error} title="Whoops..." component={Message.ErrorPage} />
        <Route path={cfg.routes.info} title="Info" component={Message.InfoPage}/>
        <Route path={cfg.routes.login} onEnter={initLogin} title="Login" component={LoginContainer}/>
        <Route path={cfg.routes.newUser} component={InviteUser}/>
        <Redirect from={cfg.routes.app} to={cfg.routes.nodes}/>
        <Route path={cfg.routes.app} onEnter={ensureUser} component={AppContainer} >
          <Route path={cfg.routes.app} onEnter={initApp} >        
            <Route path={cfg.routes.sessions} title="Stored Sessions" component={Sessions}/>
            <Route path={cfg.routes.nodes} title="Nodes" component={Nodes}/>
            <Route path={cfg.routes.terminal} title="Terminal" components={{ CurrentSessionHost: TerminalHost }} />
            <Route path={cfg.routes.player} title="Player" components={{ CurrentSessionHost: PlayerHost }} />
          </Route>  
        </Route>
        <Route path="*" component={Message.NotFound} />
      </Route>
    </Router>
  </Provider>  
), document.getElementById("app"));
