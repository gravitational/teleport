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

import { delay, http, HttpResponse } from 'msw';
import { useEffect, useState } from 'react';

import cfg from 'teleport/config';
import { storageService } from 'teleport/services/storageService';

import FormLogin, { Props } from './FormLogin';

const props: Props = {
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

  return <FormLogin {...props} attempt={attempt} />;
};

export const LocalWithSso = () => {
  const ssoProvider = [
    { name: 'github', type: 'oidc', url: '' } as const,
    { name: 'google', type: 'oidc', url: '' } as const,
  ];

  return <FormLogin {...props} authProviders={ssoProvider} />;
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
      authProviders={ssoProvider}
      isPasswordlessEnabled={true}
    />
  );
};

export const LocalDisabledWithSso = () => {
  const ssoProvider = [
    { name: 'github', type: 'oidc', url: '' } as const,
    { name: 'google', type: 'oidc', url: '' } as const,
  ];

  return (
    <FormLogin
      {...props}
      authProviders={ssoProvider}
      isLocalAuthEnabled={false}
    />
  );
};

export const LocalDisabledNoSso = () => (
  <FormLogin {...props} isLocalAuthEnabled={false} />
);

export const PrimarySso = () => {
  const ssoProvider = [
    { name: 'github', displayName: 'GitHub', type: 'oidc', url: '' } as const,
    { name: 'google', displayName: 'Google', type: 'oidc', url: '' } as const,
    {
      name: 'bitbucket',
      displayName: 'Bitbucket',
      type: 'oidc',
      url: '',
    } as const,
    {
      name: 'Mission Control',
      type: 'oidc',
      url: '',
    } as const,
    {
      name: 'Okta',
      type: 'saml',
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
    <FormLogin {...props} primaryAuthType="sso" authProviders={ssoProvider} />
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
      primaryAuthType="passwordless"
      auth2faType="optional"
    />
  );
};

export const IdentifierFirst = () => {
  const ssoProviders = [
    { name: 'github', type: 'oidc', url: '' } as const,
    { name: 'google', type: 'oidc', url: '' } as const,
  ];
  const [key, setKey] = useState(0);

  useEffect(() => {
    const defaultIdentifierFirst = cfg.auth.identifierFirstLoginEnabled;
    cfg.auth.identifierFirstLoginEnabled = true;
    setKey(prev => prev + 1); // Force remount the component with new cfg state.

    return () => {
      cfg.auth.identifierFirstLoginEnabled = defaultIdentifierFirst;
    };
  }, []);

  return <FormLogin {...props} authProviders={ssoProviders} key={key} />;
};

export const IdentifierFirstRememberedUser = () => {
  const [key, setKey] = useState(0);
  const ssoProviders = [
    { name: 'github', type: 'oidc', url: '' } as const,
    { name: 'google', type: 'oidc', url: '' } as const,
  ];

  useEffect(() => {
    storageService.setRememberedSsoUsername('joe@goteleport.com');
    const defaultIdentifierFirst = cfg.auth.identifierFirstLoginEnabled;
    cfg.auth.identifierFirstLoginEnabled = true;
    setKey(prev => prev + 1); // Force remount the component with new cfg state.

    return () => {
      cfg.auth.identifierFirstLoginEnabled = defaultIdentifierFirst;
      storageService.clearRememberedSsoUsername();
    };
  }, []);

  return <FormLogin {...props} authProviders={ssoProviders} key={key} />;
};

IdentifierFirstRememberedUser.parameters = {
  msw: {
    handlers: [
      http.post(cfg.api.authConnectorsPath, async () => {
        await delay(600); // Simulate loading state
        return HttpResponse.json([connectorsResp]);
      }),
    ],
  },
};

const connectorsResp = {
  connectors: [
    {
      name: 'Okta',
      type: 'saml',
      displayName: 'Okta',
      url: 'http://localhost/okta/login/web?redirect_url=http:%2F%2Flocalhost%2Fwebconnector_id=okta',
    },
    {
      name: 'Entra',
      type: 'saml',
      displayName: 'Entra',
      url: 'http://localhost/okta/login/web?redirect_url=http:%2F%2Flocalhost%2Fwebconnector_id=okta',
    },
  ],
};
