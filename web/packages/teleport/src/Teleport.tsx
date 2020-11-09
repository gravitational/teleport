/*
Copyright 2019 Gravitational, Inc.

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
import { hot } from 'react-hot-loader/root';
import React from 'react';
import { useFavicon } from 'shared/hooks';
import ThemeProvider from 'design/ThemeProvider';
import { Router, Route, Switch } from 'teleport/components/Router';
import CatchError from 'teleport/components/CatchError';
import Authenticated from 'teleport/components/Authenticated';
import Main from './Main';
import Invite, { ResetPassword } from './Invite';
import Login, { LoginSuccess, LoginFailed } from './Login';
import AppLauncher from './AppLauncher';
import Console from './Console';
import Player from './Player';
import TeleportContextProvider from './teleportContextProvider';
import cfg from './config';

const teleportIco = require('./favicon.ico').default;

const Teleport = ({ history, children }) => {
  useFavicon(teleportIco);
  return (
    <CatchError>
      <ThemeProvider>
        <Router history={history}>
          <Switch>
            <Route
              title="Login Failed"
              path={cfg.routes.loginFailed}
              component={LoginFailed}
            />
            <Route title="Login" path={cfg.routes.login} component={Login} />
            <Route
              title="Success"
              path={cfg.routes.loginSuccess}
              component={LoginSuccess}
            />
            <Route
              title="Invite"
              path={cfg.routes.userInvite}
              component={Invite}
            />
            <Route
              title="Password Reset"
              path={cfg.routes.userReset}
              component={ResetPassword}
            />
            <Route path={cfg.routes.root}>
              <Authenticated>
                <TeleportContextProvider>
                  <Switch>
                    <Route
                      path={cfg.routes.appLauncher}
                      component={AppLauncher}
                    />
                    <Route>{renderAuthenticated(children)}</Route>
                  </Switch>
                </TeleportContextProvider>
              </Authenticated>
            </Route>
          </Switch>
        </Router>
      </ThemeProvider>
    </CatchError>
  );
};

// TODO: make it lazy loadable
function renderAuthenticated(children) {
  // this is how enterprise version renders it's own routes
  if (children) {
    return children;
  }

  return (
    <Switch>
      <Route path={cfg.routes.console} component={Console} />
      <Route path={cfg.routes.player} component={Player} />
      <Route path={cfg.routes.root} component={Main} />
    </Switch>
  );
}

export default hot(Teleport);
