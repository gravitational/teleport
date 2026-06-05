import { useTheme } from 'styled-components';

import Flex from 'design/Flex';

import cfg from 'e-teleport/config';
import LoginForm from 'teleport/components/FormLogin';
import { LogoHero } from 'teleport/components/LogoHero';
import { PoweredByTeleportLogo } from 'teleport/components/PoweredByTeleportLogo';
import Motd from 'teleport/Login/Motd';
import useLogin, { State } from 'teleport/Login/useLogin';
import history from 'teleport/services/history';

import mcLogo from '../Main/mcLogo/mcLogo.svg';
import spacexLogoDark from '../Main/spacexLogo/spacexLogoDark.svg';
import spacexLogoLight from '../Main/spacexLogo/spacexLogoLight.svg';

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
  const theme = useTheme();

  // while we are checking if a session is valid, we don't return anything
  // to prevent flickering. The check only happens for a frame or two so
  // we avoid rendering a loader/indicator since that will flicker as well
  if (checkingValidSession) {
    return null;
  }

  let isCustomForm = false;

  let logo = <LogoHero />;
  switch (cfg.oss.customTheme) {
    case 'mc':
      logo = <LogoHero customSrc={mcLogo} />;
      isCustomForm = true;
      break;
    case 'spacex':
      logo = (
        <LogoHero
          customSrc={theme.name === 'dark' ? spacexLogoDark : spacexLogoLight}
        />
      );
      isCustomForm = true;
      break;
  }

  const productName = cfg.oss.getBeamsUi() ? 'Beams' : 'Teleport';
  const title = isCustomForm ? 'Sign in' : `Sign in to ${productName}`;
  const ssoTitle = isCustomForm
    ? 'Sign in with SSO'
    : `Sign in to ${productName} with SSO`;

  return (
    <>
      {logo}
      {showMotd ? (
        <Motd message={motd} onClick={acknowledgeMotd} />
      ) : (
        <>
          <LoginForm
            title={title}
            ssoTitle={ssoTitle}
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
          {isCustomForm && (
            <Flex alignItems="center" justifyContent="center">
              <PoweredByTeleportLogo />
            </Flex>
          )}
        </>
      )}
    </>
  );
}
