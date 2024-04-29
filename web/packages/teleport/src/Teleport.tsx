/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import React, { Suspense } from 'react';
import ThemeProvider from 'design/ThemeProvider';

import { Route, Router, Switch } from 'teleport/components/Router';
import { CatchError } from 'teleport/components/CatchError';
import Authenticated from 'teleport/components/Authenticated';

import { getOSSFeatures } from 'teleport/features';

import { LayoutContextProvider } from 'teleport/Main/LayoutContext';
import { UserContextProvider } from 'teleport/User';
import { NewCredentials } from 'teleport/Welcome/NewCredentials';

import TeleportContextProvider from './TeleportContextProvider';
import TeleportContext from './teleportContext';
import cfg from './config';
import { AppLauncher } from './AppLauncher';
import { LoginFailedComponent as LoginFailed } from './Login/LoginFailed';
import { LoginSuccess } from './Login/LoginSuccess';
import { LoginTerminalRedirect } from './Login/LoginTerminalRedirect';
import { LoginClose } from './Login/LoginClose';
import { Login } from './Login';
import { Welcome } from './Welcome';

import { ConsoleWithContext as Console } from './Console';
import { Player } from './Player';
import { DesktopSessionContainer as DesktopSession } from './DesktopSession';

import { HeadlessRequest } from './HeadlessRequest';

import { Main } from './Main';

import type { History } from 'history';

const Teleport: React.FC<Props> = props => {
  const { ctx, history } = props;
  const createPublicRoutes = props.renderPublicRoutes || publicOSSRoutes;
  const createPrivateRoutes = props.renderPrivateRoutes || privateOSSRoutes;

  return (
    <CatchError>
      <ThemeProvider>
        <LayoutContextProvider>
          <Router history={history}>
            <Suspense fallback={null}>
              <Switch>
                {createPublicRoutes()}
                <Route path={cfg.routes.root}>
                  <Authenticated>
                    <UserContextProvider>
                      <TeleportContextProvider ctx={ctx}>
                        <Switch>
                          <Route
                            path={cfg.routes.appLauncher}
                            component={AppLauncher}
                          />
                          <Route>{createPrivateRoutes()}</Route>
                        </Switch>
                      </TeleportContextProvider>
                    </UserContextProvider>
                  </Authenticated>
                </Route>
              </Switch>
            </Suspense>
          </Router>
        </LayoutContextProvider>
      </ThemeProvider>
    </CatchError>
  );
};

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
      key="terminal"
      title="Finish Login in Terminal"
      path={cfg.routes.loginTerminalRedirect}
      component={LoginTerminalRedirect}
    />,
    <Route
      key="autoclose"
      title="Working on SSO login"
      path={cfg.routes.loginClose}
      component={LoginClose}
    />,
    <Route
      key="invite"
      title="Invite"
      path={cfg.routes.userInvite}
      render={() => <Welcome NewCredentials={NewCredentials} />}
    />,
    <Route
      key="password-reset"
      title="Password Reset"
      path={cfg.routes.userReset}
      render={() => <Welcome NewCredentials={NewCredentials} />}
    />,
  ];
}

function privateOSSRoutes() {
  return (
    <Switch>
      {getSharedPrivateRoutes()}
      <Route
        path={cfg.routes.root}
        render={() => <Main features={getOSSFeatures()} />}
      />
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
    <Route
      key="headlessSSO"
      path={cfg.routes.headlessSso}
      component={HeadlessRequest}
    />,
  ];
}

export default Teleport;

export type Props = {
  ctx: TeleportContext;
  history: History;
  renderPublicRoutes?: () => React.ReactNode[];
  renderPrivateRoutes?: () => React.ReactNode;
};
