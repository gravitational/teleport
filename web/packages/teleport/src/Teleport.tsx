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

import type { History } from 'history';
import React, { Suspense, useEffect } from 'react';

import Authenticated from 'teleport/components/Authenticated';
import { CatchError } from 'teleport/components/CatchError';
import { Route, Router, Switch } from 'teleport/components/Router';
import { getOSSFeatures } from 'teleport/features';
import { LayoutContextProvider } from 'teleport/Main/LayoutContext';
import { ThemeProvider, updateFavicon } from 'teleport/ThemeProvider';
import { UserContextProvider } from 'teleport/User';
import { NewCredentials } from 'teleport/Welcome/NewCredentials';

import { AppLauncher } from './AppLauncher';
import cfg from './config';
import { ConsoleWithContext as Console } from './Console';
import { DesktopSessionContainer as DesktopSession } from './DesktopSession';
import { HeadlessRequest } from './HeadlessRequest';
import { Login } from './Login';
import { LoginClose } from './Login/LoginClose';
import { LoginFailedComponent as LoginFailed } from './Login/LoginFailed';
import { LoginSuccess } from './Login/LoginSuccess';
import { LoginTerminalRedirect } from './Login/LoginTerminalRedirect';
import { Main } from './Main';
import { Player } from './Player';
import { SingleLogoutFailed } from './SingleLogoutFailed';
import TeleportContext from './teleportContext';
import TeleportContextProvider from './TeleportContextProvider';
import { Welcome } from './Welcome';

const Teleport: React.FC<Props> = props => {
  const { ctx, history } = props;
  const createPublicRoutes = props.renderPublicRoutes || publicOSSRoutes;
  const createPrivateRoutes = props.renderPrivateRoutes || privateOSSRoutes;
  // update the favicon based on the system pref, and listen if it changes
  // overtime.
  // TODO(avatus) this can be expanded upon eventually to handle the entire theme
  // once we have a user settings page that allows users to properly set their theme
  // to respect the system prefs. We only update the favicon here because the selected theme
  // of the page doesn't necessarily match the theme of the browser, which is what we
  // are trying to match.
  useEffect(() => {
    updateFavicon();

    const colorSchemeQueryList = window.matchMedia(
      '(prefers-color-scheme: dark)'
    );

    const colorSchemeListener = () => {
      updateFavicon();
    };

    colorSchemeQueryList.addEventListener('change', colorSchemeListener);

    return () => {
      colorSchemeQueryList.removeEventListener('change', colorSchemeListener);
    };
  }, []);

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
    <Route
      key="saml-slo-failed"
      title="SAML Single Logout Failed"
      path={cfg.routes.samlSloFailed}
      component={SingleLogoutFailed}
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
