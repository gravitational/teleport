import React from 'react';
import LoginForm from 'teleport/components/FormLogin';
import Logo from 'teleport/components/LogoHero';
import history from 'teleport/services/history';
import useLogin, { State } from 'teleport/Login/useLogin';

import logoSrc from 'design/assets/images/teleport-medallion.svg';

import Motd from 'teleport/Login/Motd';

import cfg from 'e-teleport/config';

export default function Container() {
  const state = useLogin() as State;
  state.isRecoveryEnabled = cfg.oss.recoveryCodesEnabled;
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
  motd,
  showMotd,
  acknowledgeMotd,
}: State) {
  return (
    <>
      <Logo src={logoSrc} />
      {showMotd ? (
        <Motd message={motd} onClick={acknowledgeMotd} />
      ) : (
        <LoginForm
          title={'Sign in to Teleport'}
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
        />
      )}
    </>
  );
}
