import { useEffect, useState } from 'react';

import cfg from 'teleport/config';
import { State } from 'teleport/Login/useLogin';

import { Login } from './Login';

const sample: State = {
  attempt: {
    isProcessing: false,
    isFailed: false,
    isSuccess: true,
    message: '',
  },
  checkingValidSession: false,
  onLogin: () => null,
  onLoginWithWebauthn: () => null,
  onLoginWithSso: () => null,
  authProviders: [],
  auth2faType: 'off',
  preferredMfaType: 'webauthn',
  isLocalAuthEnabled: true,
  clearAttempt: () => null,
  isPasswordlessEnabled: false,
  primaryAuthType: 'local',
  motd: '',
  showMotd: false,
  acknowledgeMotd: () => null,
  licenseAcknowledged: true,
  setLicenseAcknowledged: () => {},
};

const ssoProviders = [
  { name: 'github', type: 'oidc', url: '' } as const,
  { name: 'google', type: 'oidc', url: '' } as const,
];

function useBeamsUi({ identifierFirstLogin = false } = {}) {
  const [key, setKey] = useState(0);

  useEffect(() => {
    const previousBeamsUi = cfg.beamsUi;
    const previousIdentifierFirst = cfg.auth.identifierFirstLoginEnabled;
    cfg.beamsUi = true;
    if (identifierFirstLogin) {
      cfg.auth.identifierFirstLoginEnabled = true;
    }
    setKey(k => k + 1);
    return () => {
      cfg.beamsUi = previousBeamsUi;
      cfg.auth.identifierFirstLoginEnabled = previousIdentifierFirst;
    };
  }, [identifierFirstLogin]);

  return key;
}

export default {
  title: 'TeleportE/Login',
};

export const Default = () => <Login {...sample} />;

export const Beams = () => {
  const key = useBeamsUi();
  return <Login {...sample} key={key} />;
};

export const BeamsWithSso = () => {
  const key = useBeamsUi();
  return (
    <Login
      {...sample}
      authProviders={ssoProviders}
      primaryAuthType="sso"
      key={key}
    />
  );
};

export const BeamsIdentifierFirst = () => {
  const key = useBeamsUi({ identifierFirstLogin: true });
  return <Login {...sample} authProviders={ssoProviders} key={key} />;
};
