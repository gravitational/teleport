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
import LoginForm from 'shared/components/FormLogin';
import * as actions from 'gravity/flux/user/actions';
import cfg from 'gravity/config';
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

    return (
      <>
        <Logo src={logoSrc} />
        <LoginForm
          title={cfg.user.login.headerText}
          authProviders={authProviders}
          auth2faType={auth2faType}
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

  function onLogin(...params) {
    attemptActions.start();
    actions.login(...params).fail(err => {
      attemptActions.error(err);
    });
  }

  function onLoginWithU2f(...params) {
    attemptActions.start();
    actions.loginWithU2f(...params).fail(err => {
      attemptActions.error(err);
    });
  }

  function onLoginWithSso(...params) {
    attemptActions.start();
    actions.loginWithSso(...params);
  }

  return {
    logoSrc: cfg.user.logo,
    attempt,
    onLogin,
    onLoginWithU2f,
    onLoginWithSso,
  };
}

export default withState(mapState)(Login);
