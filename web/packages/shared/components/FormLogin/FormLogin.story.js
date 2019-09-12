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
import { storiesOf } from '@storybook/react';
import FormLogin from './FormLogin';
import { AuthProviderTypeEnum } from './../../services/enums';

const defaultProps = {
  attempt: {
    isFailed: false,
  },
  cb() {},
};

storiesOf('Shared/FormLogin', module)
  .add('with user name and password', () => {
    return (
      <FormLogin
        title="Custom Title"
        authProviders={[]}
        auth2faType="otp"
        onLoginWithSso={defaultProps.cb}
        onLoginWithU2f={defaultProps.cb}
        onLogin={defaultProps.cb}
        attempt={defaultProps.attempt}
      />
    );
  })
  .add('with server errors', () => {
    const attempt = {
      ...defaultProps.attempt,
      isFailed: true,
      message:
        'invalid credentials with looooooooooooooooooooooooooooooooong text',
    };

    return (
      <FormLogin
        title="Welcome!"
        authProviders={[]}
        auth2faType="off"
        onLoginWithSso={defaultProps.cb}
        onLoginWithU2f={defaultProps.cb}
        onLogin={defaultProps.cb}
        attempt={attempt}
      />
    );
  })
  .add('with social', () => {
    const ssoProvider = [
      { name: 'github', type: AuthProviderTypeEnum.OIDC, url: '' },
      { name: 'google', type: AuthProviderTypeEnum.OIDC, url: '' },
      { name: 'bitbucket', type: AuthProviderTypeEnum.OIDC, url: '' },
      { name: 'unknown', type: AuthProviderTypeEnum.OIDC, url: '' },
      { name: 'microsoft', type: AuthProviderTypeEnum.OIDC, url: '' },
    ];

    return (
      <FormLogin
        title="Welcome!"
        authProviders={ssoProvider}
        auth2faType="off"
        onLoginWithSso={defaultProps.cb}
        onLoginWithU2f={defaultProps.cb}
        onLogin={defaultProps.cb}
        attempt={defaultProps.attempt}
      />
    );
  })
  .add('with U2F USB KEY', () => {
    const attempt = {
      ...defaultProps.attempt,
      isProcessing: true,
    };

    return (
      <FormLogin
        title="Welcome!"
        authProviders={[]}
        auth2faType="u2f"
        onLoginWithSso={defaultProps.cb}
        onLoginWithU2f={defaultProps.cb}
        onLogin={defaultProps.cb}
        attempt={attempt}
      />
    );
  });
