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
import { Router, IndexRoute, Route, Redirect } from 'react-router';
import { Provider } from 'nuclear-js-react-addons';

// telebase imports
import { ensureUser, initLogin } from 'telebase-app/flux/user/actions';
import { initApp } from 'telebase-app/flux/app/actions';
import history from 'telebase-app/services/history';
import LoginContainer from 'telebase-app/components/user/login.jsx';
import Signup from 'telebase-app/components/user/invite.jsx';
import Nodes from 'telebase-app/components/nodes/main.jsx';
import Sessions from 'telebase-app/components/sessions/main.jsx';
import TerminalHost from 'telebase-app/components/terminal/terminalHost.jsx';
import PlayerHost from 'telebase-app/components/player/playerHost.jsx';
import { MessagePage, NotFound } from 'telebase-app/components/msgPage.jsx';
import DocumentTitle from 'telebase-app/components/documentTitle';
import 'telebase-app/flux';

// app imports
import reactor from 'app/reactor';
import cfg from 'app/config';
import Settings from './components/settings/main';
import SettingsRoles from './components/settings/roles/main';
import SettingsAuth from './components/settings/auth/main';
import SettingsClustering from './components/settings/clustering/main';
import App from './components/app.jsx';
import { initSettings } from 'app/flux/settings/actions';
import './flux';

history.init();
cfg.init(window.GRV_CONFIG);

render((  
  <Provider reactor={reactor}>        
    <Router history={history.original()}>      
      <Route component={DocumentTitle}>
        <Route path={cfg.routes.msgs} title="Whoops" component={MessagePage}/>
        <Route path={cfg.routes.login} onEnter={initLogin} title="Login" component={LoginContainer}/>
        <Route path={cfg.routes.newUser} component={Signup}/>
        <Redirect from={cfg.routes.app} to={cfg.routes.nodes}/>
        <Route path={cfg.routes.app} onEnter={ensureUser} component={App} >      
          <Route onEnter={initApp} >        
            <Route path={cfg.routes.settingsBase} onEnter={initSettings} component={Settings}>
              <IndexRoute component={SettingsAuth} />  
              <Route path={cfg.routes.settingsRoles} component={SettingsRoles} />
              <Route path={cfg.routes.settingsCluster} component={SettingsClustering} />            
            </Route>            
            <Route path={cfg.routes.sessions} title="Stored Sessions" component={Sessions}/>
            <Route path={cfg.routes.nodes} title="Nodes" component={Nodes}/>
            <Route path={cfg.routes.terminal} title="Terminal" components={{ CurrentSessionHost: TerminalHost }} />
            <Route path={cfg.routes.player} title="Stored Sessions" components={{ CurrentSessionHost: PlayerHost }} />
          </Route>        
        </Route>
        <Route path="*" component={NotFound} />
      </Route>  
    </Router>
  </Provider>
), document.getElementById("app"));