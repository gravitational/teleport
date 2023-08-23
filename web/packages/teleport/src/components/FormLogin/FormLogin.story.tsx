/*
Copyright 2019-2022 Gravitational, Inc.

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

import FormLogin, { Props } from './FormLogin';

const props: Props = {
  title: 'Custom Title',
  attempt: {
    isFailed: false,
    isSuccess: false,
    isProcessing: false,
    message: '',
  },
  authProviders: [],
  onLoginWithSso: () => null,
  onLoginWithWebauthn: () => null,
  onLogin: () => null,
  clearAttempt: () => null,
  auth2faType: 'off',
  primaryAuthType: 'local',
  isPasswordlessEnabled: false,
  privateKeyPolicyEnabled: false,
};

export default {
  title: 'Teleport/FormLogin',
};

export const LocalOnly = () => <FormLogin {...props} />;

export const LocalWithOtp = () => <FormLogin {...props} auth2faType="otp" />;

export const LocalWithWebauthn = () => (
  <FormLogin {...props} auth2faType="webauthn" />
);

export const LocalWithOptional = () => (
  <FormLogin {...props} auth2faType="optional" />
);

export const LocalProcessing = () => (
  <FormLogin
    {...props}
    auth2faType="optional"
    attempt={{
      isProcessing: true,
      isFailed: false,
      isSuccess: false,
      message: '',
    }}
  />
);

export const LocalWithOnAndPwdless = () => (
  <FormLogin {...props} auth2faType="on" isPasswordlessEnabled={true} />
);

export const Cloud = () => (
  <FormLogin
    {...props}
    title="Teleport Cloud"
    auth2faType="on"
    isRecoveryEnabled={true}
    onRecover={() => null}
  />
);

export const ServerError = () => {
  const attempt = {
    ...props.attempt,
    isFailed: true,
    message:
      'invalid credentials with looooooooooooooooooooooooooooooooong text',
  };

  return <FormLogin {...props} title="Welcome!" attempt={attempt} />;
};

export const LocalWithSso = () => {
  const ssoProvider = [
    { name: 'github', type: 'oidc', url: '' } as const,
    { name: 'google', type: 'oidc', url: '' } as const,
  ];

  return <FormLogin {...props} title="Welcome!" authProviders={ssoProvider} />;
};

export const LocalWithSsoAndPwdless = () => {
  const ssoProvider = [
    {
      displayName: 'github',
      name: 'github',
      type: 'oidc',
      url: '',
    } as const,
  ];

  return (
    <FormLogin
      {...props}
      title="Welcome!"
      authProviders={ssoProvider}
      isPasswordlessEnabled={true}
    />
  );
};

export const PrivateKeyPolicyEnabled = () => (
  <FormLogin {...props} privateKeyPolicyEnabled={true} />
);

export const LocalDisabledWithSso = () => {
  const ssoProvider = [
    { name: 'github', type: 'oidc', url: '' } as const,
    { name: 'google', type: 'oidc', url: '' } as const,
  ];

  return (
    <FormLogin
      {...props}
      title="Welcome!"
      authProviders={ssoProvider}
      isLocalAuthEnabled={false}
    />
  );
};

export const LocalDisabledNoSso = () => (
  <FormLogin {...props} title="Welcome!" isLocalAuthEnabled={false} />
);

export const PrimarySso = () => {
  const ssoProvider = [
    { name: 'github', type: 'oidc', url: '' } as const,
    { name: 'google', type: 'oidc', url: '' } as const,
    { name: 'bitbucket', type: 'oidc', url: '' } as const,
    {
      name: 'Mission Control',
      type: 'oidc',
      url: '',
    } as const,
    {
      displayName: 'Microsoft',
      name: 'microsoft',
      type: 'oidc',
      url: '',
    } as const,
  ];

  return (
    <FormLogin
      {...props}
      title="Welcome!"
      primaryAuthType="sso"
      authProviders={ssoProvider}
    />
  );
};

export const PrimarySsoWithPwdless = () => {
  const ssoProvider = [
    { name: 'github', type: 'oidc', url: '' } as const,
    { name: 'google', type: 'oidc', url: '' } as const,
  ];

  return (
    <FormLogin
      {...props}
      title="Welcome!"
      primaryAuthType="sso"
      authProviders={ssoProvider}
      isPasswordlessEnabled={true}
    />
  );
};

export const PrimarySsoWithSecondFactor = () => {
  const ssoProvider = [
    { name: 'github', type: 'oidc', url: '' } as const,
    { name: 'google', type: 'oidc', url: '' } as const,
  ];

  return (
    <FormLogin
      {...props}
      title="Welcome!"
      primaryAuthType="sso"
      auth2faType="on"
      authProviders={ssoProvider}
    />
  );
};

export const PrimaryPwdless = () => {
  const ssoProvider = [
    { name: 'github', type: 'oidc', url: '' } as const,
    { name: 'google', type: 'oidc', url: '' } as const,
  ];

  return (
    <FormLogin
      {...props}
      title="Welcome!"
      primaryAuthType="passwordless"
      auth2faType="webauthn"
      authProviders={ssoProvider}
    />
  );
};

export const PrimaryPwdlessWithNoSso = () => {
  return (
    <FormLogin
      {...props}
      title="Welcome!"
      primaryAuthType="passwordless"
      auth2faType="optional"
    />
  );
};
