import React from 'react';
import LoginForm from 'teleport/components/FormLogin';
import Logo from 'teleport/components/LogoHero';
import history from 'teleport/services/history';
import useLogin, { State } from 'teleport/Login/useLogin';

import cfg from 'e-teleport/config';

const logoSrc = require('design/assets/images/teleport-medallion.svg');

export default function Container() {
  const state = useLogin() as State;
  state.isRecoveryEnabled = cfg.oss.isCloud;
  state.onRecover = (isRecoverPassword: boolean) => {
    const path = isRecoverPassword
      ? cfg.routes.recoveryForgotPassword
      : cfg.routes.recoveryForgotDevice;
    history.push(path);
  };
  return <Login {...state} />;
}

export function Login({
  attempt,
  onLogin,
  onLoginWithWebauthn,
  onLoginWithSso,
  authProviders,
  auth2faType,
  preferredMfaType,
  isLocalAuthEnabled,
  isRecoveryEnabled,
  onRecover,
  clearAttempt,
  isPasswordlessEnabled,
  primaryAuthType,
  privateKeyPolicyEnabled,
}: State) {
  return (
    <>
      <Logo src={logoSrc} />
      <LoginForm
        title={'Sign into Teleport'}
        authProviders={authProviders}
        auth2faType={auth2faType}
        preferredMfaType={preferredMfaType}
        isLocalAuthEnabled={isLocalAuthEnabled}
        onLoginWithSso={onLoginWithSso}
        onLoginWithWebauthn={onLoginWithWebauthn}
        onLogin={onLogin}
        attempt={attempt}
        clearAttempt={clearAttempt}
        isRecoveryEnabled={isRecoveryEnabled}
        onRecover={onRecover}
        isPasswordlessEnabled={isPasswordlessEnabled}
        primaryAuthType={primaryAuthType}
        privateKeyPolicyEnabled={privateKeyPolicyEnabled}
      />
    </>
  );
}
