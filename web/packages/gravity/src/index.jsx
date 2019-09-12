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

// oss imports
import { Invite, PasswordReset } from 'gravity/components/Invite';
import { FluxContext } from 'gravity/components/nuclear';
import CatchError from 'gravity/components/CatchError';
import Login, { LoginSuccess, LoginFailed } from 'gravity/components/Login';
import reactor from 'gravity/reactor';
import Console from 'gravity/console';
import { Redirect, Router, Route, Switch } from 'gravity/components/Router';
import { withAuth } from 'gravity/components/Hocs';
import 'gravity/flux';
const Installer = React.lazy(() => import('gravity/installer'));

// local imports
import cfg from './config';
import ClusterApp from './cluster';

const Authorized = withAuth(() => (
  <Switch>
    <Route path={cfg.routes.console} component={Console} />
    <Route path={cfg.routes.siteBase} component={ClusterApp} />
    <Route path={cfg.routes.installerBase} component={LazyLoad(Installer)} />
  </Switch>
));

const Root = ({ history }) => (
  <CatchError>
    <FluxContext.Provider value={reactor}>
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
              component={PasswordReset}
            />
            <Redirect
              exact
              from={cfg.routes.app}
              to={cfg.routes.defaultEntry}
            />
            <Route path={cfg.routes.app} component={Authorized} />
          </Switch>
        </Router>
      </ThemeProvider>
    </FluxContext.Provider>
  </CatchError>
);

function LazyLoad(Component) {
  return props => (
    <React.Suspense fallback={null}>
      <Component {...props} />
    </React.Suspense>
  );
}

export default hot(Root);
