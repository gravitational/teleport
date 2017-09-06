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

import cfg from './config';
import { ensureUser } from './flux/user/actions';
import LoginContainer from './components/user/login.jsx';
import InviteUser from './components/user/invite.jsx';
import * as Message from './components/msgPage.jsx';
import DocumentTitle from './components/documentTitle';
import App from './components/app.jsx';

export function withAllRoutes(otherRoutes = []) {  
  return [{
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
        component: App,        
        childRoutes: otherRoutes      
        
      },
      { path: '*', component: Message.NotFound }
    ]
  }];
}