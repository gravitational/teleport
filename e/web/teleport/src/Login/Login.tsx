import React from 'react';
import LoginForm from 'shared/components/FormLogin';
import history from 'teleport/services/history';
import Logo from 'teleport/components/LogoHero';
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
  onLoginWithU2f,
  onLoginWithSso,
  authProviders,
  auth2faType,
  isLocalAuthEnabled,
  isRecoveryEnabled,
  onRecover,
  clearAttempt,
}: State) {
  return (
    <>
      <Logo src={logoSrc} />
      <LoginForm
        title={'Sign into Teleport'}
        authProviders={authProviders}
        auth2faType={auth2faType}
        isLocalAuthEnabled={isLocalAuthEnabled}
        onLoginWithSso={onLoginWithSso}
        onLoginWithU2f={onLoginWithU2f}
        onLogin={onLogin}
        attempt={attempt}
        clearAttempt={clearAttempt}
        isRecoveryEnabled={isRecoveryEnabled}
        onRecover={onRecover}
      />
    </>
  );
}
