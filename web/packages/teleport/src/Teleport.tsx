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

import React, { Suspense } from 'react';
import ThemeProvider from 'design/ThemeProvider';

import { Router, Route, Switch } from 'teleport/components/Router';
import { CatchError } from 'teleport/components/CatchError';
import Authenticated from 'teleport/components/Authenticated';

import { FeaturesContextProvider } from 'teleport/FeaturesContext';

import { getOSSFeatures } from 'teleport/features';

import { Feature } from 'teleport/types';

import { Main } from './Main';

import TeleportContextProvider from './TeleportContextProvider';
import TeleportContext from './teleportContext';
import cfg from './config';

import type { History } from 'history';

const AppLauncher = React.lazy(
  () => import(/* webpackChunkName: "app-launcher" */ './AppLauncher')
);

const Teleport: React.FC<Props> = props => {
  const { ctx, history } = props;
  const publicRoutes = props.renderPublicRoutes || renderPublicRoutes;
  const privateRoutes = props.renderPrivateRoutes || renderPrivateRoutes;

  const features = props.features || getOSSFeatures();

  return (
    <CatchError>
      <ThemeProvider>
        <Router history={history}>
          <Switch>
            {publicRoutes()}
            <Route path={cfg.routes.root}>
              <Authenticated>
                <TeleportContextProvider ctx={ctx}>
                  <FeaturesContextProvider value={features}>
                    <Switch>
                      <Route
                        path={cfg.routes.appLauncher}
                        component={AppLauncher}
                      />
                      <Route>{privateRoutes()}</Route>
                    </Switch>
                  </FeaturesContextProvider>
                </TeleportContextProvider>
              </Authenticated>
            </Route>
          </Switch>
        </Router>
      </ThemeProvider>
    </CatchError>
  );
};

const LoginFailed = React.lazy(
  () => import(/* webpackChunkName: "login-failed" */ './Login/LoginFailed')
);
const LoginSuccess = React.lazy(
  () => import(/* webpackChunkName: "login-success" */ './Login/LoginSuccess')
);
const Login = React.lazy(
  () => import(/* webpackChunkName: "login" */ './Login')
);
const Welcome = React.lazy(
  () => import(/* webpackChunkName: "welcome" */ './Welcome')
);

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
      component={Welcome}
    />,
    <Route
      key={6}
      title="Password Reset"
      path={cfg.routes.userReset}
      component={Welcome}
    />,
  ];
}

const Console = React.lazy(
  () => import(/* webpackChunkName: "console" */ './Console')
);
const Player = React.lazy(
  () => import(/* webpackChunkName: "player" */ './Player')
);
const DesktopSession = React.lazy(
  () => import(/* webpackChunkName: "desktop-session" */ './DesktopSession')
);
const Discover = React.lazy(
  () => import(/* webpackChunkName: "discover" */ './Discover')
);

// TODO: make it lazy loadable
export function renderPrivateRoutes(
  CustomMain = Main,
  CustomDiscover = Discover
) {
  return (
    <Suspense fallback={null}>
      <Switch>
        <Route path={cfg.routes.discover} component={CustomDiscover} />
        <Route path={cfg.routes.desktop} component={DesktopSession} />
        <Route path={cfg.routes.console} component={Console} />
        <Route path={cfg.routes.player} component={Player} />
        <Route path={cfg.routes.root} component={CustomMain} />
      </Switch>
    </Suspense>
  );
}

export default Teleport;

export type Props = {
  features?: Feature[];
  ctx: TeleportContext;
  history: History;
  renderPublicRoutes?(children?: JSX.Element[]): JSX.Element[];
  renderPrivateRoutes?(CustomMain?: JSX.Element): JSX.Element;
};
