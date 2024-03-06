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

import React from 'react';
import { LoginFailed as CardFailed } from 'design/CardError';

import { Route, Switch } from 'teleport/components/Router';
import LogoHero from 'teleport/components/LogoHero';
import cfg from 'teleport/config';

export function LoginFailed() {
  return (
    <Switch>
      <Route path={cfg.routes.loginErrorCallback}>
        <LoginFailedComponent message="unable to process callback" />
      </Route>
      <Route path={cfg.routes.loginErrorUnauthorized}>
        <LoginFailedComponent message="You are not authorized, please contact your SSO administrator." />
      </Route>
      <Route component={LoginFailed} />
    </Switch>
  );
}

export function LoginFailedComponent({ message }: { message?: string }) {
  const defaultMsg = "unable to login, please check Teleport's log for details";
  return (
    <>
      <LogoHero />
      <CardFailed loginUrl={cfg.routes.login} message={message || defaultMsg} />
    </>
  );
}
