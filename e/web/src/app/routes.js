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

// telebase imports
import { ensureUser } from 'telebase-app/flux/user/actions';
import LoginContainer from 'telebase-app/components/user/login.jsx';
import InviteUser from 'telebase-app/components/user/invite.jsx';
import Nodes from 'telebase-app/components/nodes/main.jsx';
import Sessions from 'telebase-app/components/sessions/main.jsx';
import TerminalHost from 'telebase-app/components/terminal/terminalHost.jsx';
import PlayerHost from 'telebase-app/components/player/playerHost.jsx';
import * as Message from 'telebase-app/components/msgPage.jsx';
import DocumentTitle from 'telebase-app/components/documentTitle';
import { initApp } from 'telebase-app/flux/app/actions';
import 'telebase-app/flux';

// app imports
import cfg from 'app/config';
import App from './components/app.jsx';
import featureRoutes from './features'

const rootRoutes = [{
  component: DocumentTitle,
  childRoutes: [
    { path: cfg.routes.error, title: "Error", component: Message.ErrorPage },
    { path: cfg.routes.info, title: "Info", component: Message.InfoPage },
    { path: cfg.routes.login, title: "Login", component: LoginContainer },
    { path: cfg.routes.newUser, component: InviteUser },
    { path: cfg.routes.app, onEnter: (localtion, replace) => replace(cfg.routes.nodes) },
    { 
      path: cfg.routes.app, 
      onEnter: ensureUser, 
      component:App,
      childRoutes: [
        { 
          path: cfg.routes.app, 
          onEnter: initApp,
          childRoutes: [
            ...featureRoutes,        
            { path: cfg.routes.sessions, title: "Stored Sessions", component: Sessions },
            { path: cfg.routes.nodes, title: "Nodes", component: Nodes  },
            { path: cfg.routes.terminal, title: "Terminal", components: { CurrentSessionHost: TerminalHost }},
            { path: cfg.routes.player, title: "Player", components: { CurrentSessionHost: PlayerHost }}
          ]
        }
      ]        
    },
    { path: '*', component: Message.NotFound }
  ]
}]
    
export default rootRoutes;