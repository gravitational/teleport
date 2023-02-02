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

import React from 'react';
import { LoginFailed as CardFailed } from 'design/CardError';

import { Route, Switch } from 'teleport/components/Router';
import LogoHero from 'teleport/components/LogoHero';
import cfg from 'teleport/config';

export default function Container() {
  return (
    <Switch>
      <Route path={cfg.routes.loginErrorCallback}>
        <LoginFailed message="unable to process callback" />
      </Route>
      <Route path={cfg.routes.loginErrorUnauthorized}>
        <LoginFailed message="You are not authorized, please contact your SSO administrator." />
      </Route>
      <Route component={LoginFailed} />
    </Switch>
  );
}

export function LoginFailed({ message }: { message?: string }) {
  const defaultMsg = "unable to login, please check Teleport's log for details";
  return (
    <>
      <LogoHero />
      <CardFailed loginUrl={cfg.routes.login} message={message || defaultMsg} />
    </>
  );
}
