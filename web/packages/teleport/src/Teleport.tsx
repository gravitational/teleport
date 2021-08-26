/*
Copyright 2019-2021 Gravitational, Inc.

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

// eslint-disable-next-line import/named
import { History } from 'history';
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
import TeleportContext from './teleportContext';
import cfg from './config';

const teleportIco = require('./favicon.ico').default;

const Teleport: React.FC<Props> = props => {
  useFavicon(teleportIco);
  const { ctx, history } = props;
  const publicRoutes = props.renderPublicRoutes || renderPublicRoutes;
  const privateRoutes = props.renderPrivateRoutes || renderPrivateRoutes;

  return (
    <CatchError>
      <ThemeProvider>
        <Router history={history}>
          <Switch>
            {publicRoutes()}
            <Route path={cfg.routes.root}>
              <Authenticated>
                <TeleportContextProvider ctx={ctx}>
                  <Switch>
                    <Route
                      path={cfg.routes.appLauncher}
                      component={AppLauncher}
                    />
                    <Route>{privateRoutes()}</Route>
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

export function renderPublicRoutes(children = []) {
  return [
    ...children,
    <Route
      key={1}
      title="Login Failed"
      path={cfg.routes.loginError}
      component={LoginFailed}
    />,
    <Route
      key={2}
      title="Login Failed"
      path={cfg.routes.loginErrorLegacy}
      component={LoginFailed}
    />,
    <Route key={3} title="Login" path={cfg.routes.login} component={Login} />,
    <Route
      key={4}
      title="Success"
      path={cfg.routes.loginSuccess}
      component={LoginSuccess}
    />,
    <Route
      key={5}
      title="Invite"
      path={cfg.routes.userInvite}
      component={Invite}
    />,
    <Route
      key={6}
      title="Password Reset"
      path={cfg.routes.userReset}
      component={ResetPassword}
    />,
  ];
}

// TODO: make it lazy loadable
export function renderPrivateRoutes(CustomMain = Main) {
  return (
    <Switch>
      <Route path={cfg.routes.desktop} component={Console} />
      <Route path={cfg.routes.console} component={Console} />
      <Route path={cfg.routes.player} component={Player} />
      <Route path={cfg.routes.root} component={CustomMain} />
    </Switch>
  );
}

export default hot(Teleport);

export type Props = {
  ctx: TeleportContext;
  history: History;
  renderPublicRoutes?(children?: JSX.Element[]): JSX.Element[];
  renderPrivateRoutes?(CustomMain?: JSX.Element): JSX.Element;
};
