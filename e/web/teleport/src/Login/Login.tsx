import cfg from 'e-teleport/config';
import LoginForm from 'teleport/components/FormLogin';
import { LogoHero } from 'teleport/components/LogoHero';
import Motd from 'teleport/Login/Motd';
import useLogin, { State } from 'teleport/Login/useLogin';
import history from 'teleport/services/history';

export function LoginContainer() {
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
  checkingValidSession,
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
  // while we are checking if a session is valid, we don't return anything
  // to prevent flickering. The check only happens for a frame or two so
  // we avoid rendering a loader/indicator since that will flicker as well
  if (checkingValidSession) {
    return null;
  }
  return (
    <>
      <LogoHero />
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
