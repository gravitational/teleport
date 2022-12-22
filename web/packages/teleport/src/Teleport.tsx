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

import TeleportContextProvider from './TeleportContextProvider';
import TeleportContext from './teleportContext';
import cfg from './config';

import type { History } from 'history';

const AppLauncher = React.lazy(
  () => import(/* webpackChunkName: "app-launcher" */ './AppLauncher')
);

const Teleport: React.FC<Props> = props => {
  const { ctx, history } = props;
  const createPublicRoutes = props.renderPublicRoutes || publicOSSRoutes;
  const createPrivateRoutes = props.renderPrivateRoutes || privateOSSRoutes;

  const features = props.features || getOSSFeatures();

  return (
    <CatchError>
      <ThemeProvider>
        <Router history={history}>
          <Suspense fallback={null}>
            <Switch>
              {createPublicRoutes()}
              <Route path={cfg.routes.root}>
                <Authenticated>
                  <TeleportContextProvider ctx={ctx}>
                    <FeaturesContextProvider value={features}>
                      <Switch>
                        <Route
                          path={cfg.routes.appLauncher}
                          component={AppLauncher}
                        />
                        <Route>{createPrivateRoutes()}</Route>
                      </Switch>
                    </FeaturesContextProvider>
                  </TeleportContextProvider>
                </Authenticated>
              </Route>
            </Switch>
          </Suspense>
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

const Main = React.lazy(() => import(/* webpackChunkName: "main" */ './Main'));

function publicOSSRoutes() {
  return [
    <Route
      title="Login"
      path={cfg.routes.login}
      component={Login}
      key="login"
    />,
    ...getSharedPublicRoutes(),
  ];
}

export function getSharedPublicRoutes() {
  return [
    <Route
      key="login-failed"
      title="Login Failed"
      path={cfg.routes.loginError}
      component={LoginFailed}
    />,
    <Route
      key="login-failed-legacy"
      title="Login Failed"
      path={cfg.routes.loginErrorLegacy}
      component={LoginFailed}
    />,
    <Route
      key="success"
      title="Success"
      path={cfg.routes.loginSuccess}
      component={LoginSuccess}
    />,
    <Route
      key="invite"
      title="Invite"
      path={cfg.routes.userInvite}
      component={Welcome}
    />,
    <Route
      key="password-reset"
      title="Password Reset"
      path={cfg.routes.userReset}
      component={Welcome}
    />,
  ];
}

function privateOSSRoutes() {
  return (
    <Switch>
      <Route path={cfg.routes.discover} component={Discover} />
      {getSharedPrivateRoutes()}
      <Route path={cfg.routes.root} component={Main} />
    </Switch>
  );
}

export function getSharedPrivateRoutes() {
  return [
    <Route
      key="desktop"
      path={cfg.routes.desktop}
      component={DesktopSession}
    />,
    <Route key="console" path={cfg.routes.console} component={Console} />,
    <Route key="player" path={cfg.routes.player} component={Player} />,
  ];
}

export default Teleport;

export type Props = {
  features?: Feature[];
  ctx: TeleportContext;
  history: History;
  renderPublicRoutes?: () => React.ReactNode[];
  renderPrivateRoutes?: () => React.ReactNode;
};
