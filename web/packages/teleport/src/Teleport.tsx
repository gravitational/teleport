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

import { Route, Routes } from 'react-router-dom';

import { CatchError } from 'teleport/components/CatchError';
import Authenticated from 'teleport/components/Authenticated';

import { getOSSFeatures } from 'teleport/features';

import TeleportContextProvider from './TeleportContextProvider';
import TeleportContext from './teleportContext';
import cfg from './config';

const AppLauncher = React.lazy(
  () => import(/* webpackChunkName: "app-launcher" */ './AppLauncher')
);

const Teleport: React.FC<Props> = props => {
  const { ctx } = props;
  const createPublicRoutes = props.renderPublicRoutes || publicOSSRoutes;
  const createPrivateRoutes = props.renderPrivateRoutes || privateOSSRoutes;

  return (
    <CatchError>
      <ThemeProvider>
        <Suspense fallback={null}>
          <Routes>
            {createPublicRoutes()}
            <Route
              path="*"
              element={
                <Authenticated>
                  <TeleportContextProvider ctx={ctx}>
                    <Routes>
                      <Route
                        path={cfg.routes.appLauncher}
                        element={<AppLauncher />}
                      />
                      {createPrivateRoutes()}
                    </Routes>
                  </TeleportContextProvider>
                </Authenticated>
              }
            />
          </Routes>
        </Suspense>
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
const Reset = React.lazy(
  () => import(/* webpackChunkName: "welcome" */ './Welcome/Reset')
);
const Invite = React.lazy(
  () => import(/* webpackChunkName: "invite" */ './Welcome/Invite')
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

const Main = React.lazy(() => import(/* webpackChunkName: "main" */ './Main'));

function publicOSSRoutes() {
  return (
    <Routes>
      <Route
        // title="Login"
        path={cfg.routes.login}
        element={Login}
        key="login"
      />
      {getSharedPublicRoutes()}
    </Routes>
  );
}

export function getSharedPublicRoutes() {
  return (
    <>
      <Route
        key="login-failed"
        // title="Login Failed"
        path={cfg.routes.loginError}
        element={<LoginFailed />}
      />
      <Route
        key="login-failed-legacy"
        // title="Login Failed"
        path={cfg.routes.loginErrorLegacy}
        element={<LoginFailed />}
      />
      <Route
        key="success"
        // title="Success"
        path={cfg.routes.loginSuccess}
        element={<LoginSuccess />}
      />
      <Route
        key="invite"
        // title="Invite"
        path={`${cfg.routes.userInvite}/*`}
        element={<Invite />}
      />
      <Route
        key="invite"
        // title="Invite"
        path={`${cfg.routes.userReset}/*`}
        element={<Reset />}
      />
    </>
  );
}

function privateOSSRoutes() {
  return (
    <>
      {getSharedPrivateRoutes()}
      <Route path="*" element={<Main features={getOSSFeatures()} />} />
    </>
  );
}

export function getSharedPrivateRoutes() {
  return (
    <>
      <Route
        key="desktop"
        path={cfg.routes.desktop}
        element={<DesktopSession />}
      />
      <Route key="console" path={cfg.routes.console} element={<Console />} />,
      <Route key="player" path={cfg.routes.player} element={<Player />} />,
    </>
  );
}

export default Teleport;

export type Props = {
  ctx: TeleportContext;
  renderPublicRoutes?: () => React.ReactNode;
  renderPrivateRoutes?: () => React.ReactNode;
};
