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
import ThemeProvider from 'design/ThemeProvider';
import { Router, Route, Switch } from 'shared/components/Router';
import Invite, { ResetPassword } from 'teleport/components/Invite';
import CatchError from 'teleport/components/CatchError';
import Login, { LoginSuccess, LoginFailed } from 'teleport/components/Login';
import Console from 'teleport/console';
import Authenticated from 'teleport/components/Authenticated';
import Offline from 'teleport/components/Offline';
import Dashboard from 'teleport/dashboard';
import SessionAudit from 'teleport/recordings';
import cfg from 'teleport/config';

const Index = ({ history, children }) => (
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
          <Route
            path={cfg.routes.app}
            render={() => (
              <Authenticated>
                <Switch>
                  <Route path={cfg.routes.console} component={Console} />
                  <Route
                    path={cfg.routes.sessionAudit}
                    component={SessionAudit}
                  />
                  <Route path={cfg.routes.clusterOffline} component={Offline} />
                  {children}
                  <Route path={cfg.routes.app} component={Dashboard} />
                </Switch>
              </Authenticated>
            )}
          />
        </Switch>
      </Router>
    </ThemeProvider>
  </CatchError>
);

export default hot(Index);
