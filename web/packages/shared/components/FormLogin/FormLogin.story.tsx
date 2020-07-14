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
import FormLogin from './FormLogin';

const defaultProps = {
  attempt: {
    isFailed: false,
    isSuccess: undefined,
    isProcessing: undefined,
    message: undefined,
  },
  cb() {},
};

export default {
  title: 'Shared/FormLogin',
};

export const Basic = () => (
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

export const ServerError = () => {
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
      onLoginWithSso={defaultProps.cb}
      onLoginWithU2f={defaultProps.cb}
      onLogin={defaultProps.cb}
      attempt={attempt}
    />
  );
};

export const SSOProviders = () => {
  const ssoProvider = [
    {
      displayName: 'github',
      name: 'github',
      type: 'oidc',
      url: '',
    } as const,
    {
      displayName: 'google',
      name: 'google',
      type: 'oidc',
      url: '',
    } as const,
    {
      displayName: 'bitbucket',
      name: 'bitbucket',
      type: 'oidc',
      url: '',
    } as const,
    {
      name: 'Mission Control',
      type: 'oidc',
      url: '',
    } as const,
    {
      displayName: 'microsoft',
      name: 'microsoft',
      type: 'oidc',
      url: '',
    } as const,
  ];

  return (
    <FormLogin
      title="Welcome!"
      authProviders={ssoProvider}
      onLoginWithSso={defaultProps.cb}
      onLoginWithU2f={defaultProps.cb}
      onLogin={defaultProps.cb}
      attempt={defaultProps.attempt}
    />
  );
};

export const Universal2ndFactor = () => {
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
};

export const LocalAuthDisabled = () => {
  const ssoProvider = [
    { name: 'github', type: 'oidc', url: '' } as const,
    { name: 'google', type: 'oidc', url: '' } as const,
  ];

  return (
    <FormLogin
      title="Welcome!"
      authProviders={ssoProvider}
      onLoginWithSso={defaultProps.cb}
      onLoginWithU2f={defaultProps.cb}
      onLogin={defaultProps.cb}
      attempt={defaultProps.attempt}
      isLocalAuthEnabled={false}
    />
  );
};

export const LocalAuthDisabledNoSSO = () => (
  <FormLogin
    title="Welcome!"
    onLoginWithSso={defaultProps.cb}
    onLoginWithU2f={defaultProps.cb}
    onLogin={defaultProps.cb}
    attempt={defaultProps.attempt}
    isLocalAuthEnabled={false}
  />
);
