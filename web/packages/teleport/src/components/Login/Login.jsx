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

import React from 'react';
import { useAttempt, withState } from 'shared/hooks';
import history from 'teleport/services/history';
import cfg from 'teleport/config';
import auth from 'teleport/services/auth';
import LoginForm from 'shared/components/FormLogin';
import Logo from './../LogoHero';

export class Login extends React.Component {
  onLoginWithSso = ssoProvider => {
    this.props.onLoginWithSso(ssoProvider.name, ssoProvider.url);
  };

  onLoginWithU2f = (username, password) => {
    this.props.onLoginWithU2f(username, password);
  };

  onLogin = (username, password, token) => {
    this.props.onLogin(username, password, token);
  };

  render() {
    const { attempt, logoSrc } = this.props;
    const authProviders = cfg.getAuthProviders();
    const auth2faType = cfg.getAuth2faType();
    const isLocalAuthEnabled = cfg.getLocalAuthFlag();

    return (
      <>
        <Logo src={logoSrc} />
        <LoginForm
          title={'Sign into Teleport'}
          authProviders={authProviders}
          auth2faType={auth2faType}
          isLocalAuthEnabled={isLocalAuthEnabled}
          onLoginWithSso={this.onLoginWithSso}
          onLoginWithU2f={this.onLoginWithU2f}
          onLogin={this.onLogin}
          attempt={attempt}
        />
      </>
    );
  }
}

function mapState() {
  const [attempt, attemptActions] = useAttempt();
  function onLogin(email, password, token) {
    attemptActions.start();
    auth
      .login(email, password, token)
      .then(onSuccess)
      .catch(err => {
        attemptActions.error(err);
      });
  }

  function onLoginWithU2f(name, password) {
    attemptActions.start();
    auth
      .loginWithU2f(name, password)
      .then(onSuccess)
      .catch(err => {
        attemptActions.error(err);
      });
  }

  function onLoginWithSso(providerName, redirectUrl) {
    attemptActions.start();
    const appStartRoute = getEntryRoute();
    const ssoUri = cfg.getSsoUrl(redirectUrl, providerName, appStartRoute);
    history.push(ssoUri, true);
  }

  return {
    attempt,
    onLogin,
    onLoginWithU2f,
    onLoginWithSso,
  };
}

function onSuccess() {
  const redirect = getEntryRoute();
  const withPageRefresh = true;
  history.push(redirect, withPageRefresh);
}

function getEntryRoute() {
  let entryUrl = history.getRedirectParam();
  if (entryUrl) {
    entryUrl = history.ensureKnownRoute(entryUrl);
  } else {
    entryUrl = cfg.routes.app;
  }

  return history.ensureBaseUrl(entryUrl);
}

export default withState(mapState)(Login);
