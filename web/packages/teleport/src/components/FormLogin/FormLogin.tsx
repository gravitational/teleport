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

import React, { useMemo, useState } from 'react';
import styled from 'styled-components';

import {
  Box,
  Button,
  ButtonLink,
  ButtonPrimary,
  ButtonSecondary,
  ButtonText,
  Card,
  Flex,
  Text,
} from 'design';
import * as Alerts from 'design/Alert';
import { StepComponentProps, StepSlider } from 'design/StepSlider';
import { P } from 'design/Text/Text';
import FieldInput from 'shared/components/FieldInput';
import { FieldSelect } from 'shared/components/FieldSelect';
import Validation, { Validator } from 'shared/components/Validation';
import {
  requiredField,
  requiredToken,
} from 'shared/components/Validation/rules';
import { useAttempt, useRefAutoFocus } from 'shared/hooks';
import {
  Auth2faType,
  AuthProvider,
  PreferredMfaType,
  PrimaryAuthType,
} from 'shared/services';
import createMfaOptions, { MfaOption } from 'shared/utils/createMfaOptions';

import cfg from 'teleport/config';
import { UserCredentials } from 'teleport/services/auth';
import history from 'teleport/services/history';

import { PasskeyIcons } from '../Passkeys';
import FormIdentifierFirst from './FormIdentifierFirst';
import SSOButtonList from './SsoButtons';

const allAuthTypes: PrimaryAuthType[] = ['passwordless', 'sso', 'local'];

export default function LoginForm(props: Props) {
  const {
    attempt,
    isLocalAuthEnabled = true,
    isPasswordlessEnabled,
    authProviders = [],
    primaryAuthType,
  } = props;

  const [showIdentifierFirstLogin, setShowIdentifierFirstLogin] = useState(
    cfg?.auth?.identifierFirstLoginEnabled
  );

  const ssoEnabled = authProviders?.length > 0;

  // If local auth was not enabled, disregard any primary auth type config
  // and display sso providers if any.
  const actualPrimaryType = isLocalAuthEnabled ? primaryAuthType : 'sso';

  const allowedAuthTypes = allAuthTypes.filter(t => {
    if (!isLocalAuthEnabled) return ssoEnabled && t === 'sso';
    if (!isPasswordlessEnabled && t === 'passwordless') return false;
    if (!ssoEnabled && t === 'sso') return false;
    return true;
  });
  const otherAuthTypes = allowedAuthTypes.filter(t => t !== actualPrimaryType);

  let errorMessage = '';
  if (allowedAuthTypes.length === 0) {
    errorMessage = 'Login has not been enabled';
  } else if (attempt.isFailed) {
    errorMessage = attempt.message;
  }

  const showAccessChangedMessage = history.hasAccessChangedParam();

  // Everything below requires local auth to be enabled.
  return (
    <Card my="5" mx="auto" maxWidth={500} minWidth={300} py={4}>
      <Text typography="h1" mb={4} textAlign="center">
        Sign in to Teleport
      </Text>
      {errorMessage && <Alerts.Danger m={4}>{errorMessage}</Alerts.Danger>}
      {showAccessChangedMessage && (
        <Alerts.Warning m={4}>
          Your access has changed. Please re-login.
        </Alerts.Warning>
      )}
      {allowedAuthTypes.length > 0 ? (
        <StepSlider<typeof loginViews>
          flows={loginViews}
          currFlow={'default'}
          otherAuthTypes={otherAuthTypes}
          {...props}
          showIdentifierFirstLogin={showIdentifierFirstLogin}
          setShowIdentifierFirstLogin={setShowIdentifierFirstLogin}
          primaryAuthType={actualPrimaryType}
        />
      ) : (
        <P mx={4}>
          The ability to login has not been enabled. Please contact your system
          administrator for more information.
        </P>
      )}
    </Card>
  );
}

const SsoList = ({
  attempt,
  authProviders,
  onLoginWithSso,
  autoFocus = false,
  hasTransitionEnded,
  setShowIdentifierFirstLogin,
}: Props & { hasTransitionEnded?: boolean }) => {
  const ref = useRefAutoFocus<HTMLButtonElement>({
    shouldFocus: hasTransitionEnded && autoFocus,
  });
  const { isProcessing } = attempt;

  if (cfg?.auth?.identifierFirstLoginEnabled) {
    return (
      <ButtonLink
        onClick={() => setShowIdentifierFirstLogin(true)}
        disabled={isProcessing}
      >
        Sign in using SSO
      </ButtonLink>
    );
  }

  return (
    <SSOButtonList
      isDisabled={isProcessing}
      providers={authProviders}
      onClick={onLoginWithSso}
      ref={ref}
    />
  );
};

const Passwordless = ({
  onLoginWithWebauthn,
  attempt,
  autoFocus = false,
  hasTransitionEnded,
  primary,
}: Props & { hasTransitionEnded: boolean; primary: boolean }) => {
  const ref = useRefAutoFocus<HTMLButtonElement>({
    shouldFocus: hasTransitionEnded && autoFocus,
  });
  return (
    <Box data-testid="passwordless">
      <Flex
        flexDirection="column"
        border={1}
        borderColor="interactive.tonal.neutral.2"
        borderRadius={3}
        p={3}
        gap={3}
      >
        <div>
          <PasskeyIcons />
        </div>
        <div>
          <P>Your browser will prompt you for a device key.</P>
        </div>
        <Button
          fill="filled"
          intent={primary ? 'primary' : 'neutral'}
          size="extra-large"
          setRef={ref}
          disabled={attempt.isProcessing}
          onClick={() => onLoginWithWebauthn()}
        >
          Sign in with a Passkey
        </Button>
      </Flex>
    </Box>
  );
};

const LocalForm = ({
  isRecoveryEnabled,
  onRecover,
  auth2faType,
  attempt,
  onLogin,
  onLoginWithWebauthn,
  clearAttempt,
  hasTransitionEnded,
  autoFocus = false,
}: Props & { hasTransitionEnded: boolean }) => {
  const { isProcessing } = attempt;
  const [pass, setPass] = useState('');
  const [user, setUser] = useState('');
  const [token, setToken] = useState('');

  const mfaOptions = useMemo(
    () => createMfaOptions({ auth2faType: auth2faType }),
    []
  );

  const usernameInputRef = useRefAutoFocus<HTMLInputElement>({
    shouldFocus: hasTransitionEnded && autoFocus,
  });

  const [mfaType, setMfaType] = useState(mfaOptions[0]);

  function onSetMfaOption(option: MfaOption, validator: Validator) {
    setToken('');
    clearAttempt();
    validator.reset();
    setMfaType(option);
  }

  function onLoginClick(
    e: React.MouseEvent<HTMLButtonElement>,
    validator: Validator
  ) {
    e.preventDefault();
    if (!validator.validate()) {
      return;
    }

    switch (mfaType?.value) {
      case 'webauthn':
        onLoginWithWebauthn({ username: user, password: pass });
        break;
      default:
        onLogin(user, pass, token);
    }
  }

  return (
    <Validation>
      {({ validator }) => (
        <Flex
          as="form"
          justifyContent="center"
          flexDirection="column"
          borderBottomLeftRadius="3"
          borderBottomRightRadius="3"
          data-testid="userpassword"
        >
          <FieldInput
            ref={usernameInputRef}
            rule={requiredField('Username is required')}
            label="Username"
            value={user}
            onChange={e => setUser(e.target.value)}
            placeholder="Username"
            disabled={attempt.isProcessing}
            mb={3}
          />
          <Box mb={isRecoveryEnabled ? 1 : 3}>
            <FieldInput
              rule={requiredField('Password is required')}
              label="Password"
              helperText={
                isRecoveryEnabled && (
                  <ButtonLink
                    style={{ padding: '0px', minHeight: 0 }}
                    onClick={() => onRecover(true)}
                  >
                    Forgot Password?
                  </ButtonLink>
                )
              }
              value={pass}
              onChange={e => setPass(e.target.value)}
              type="password"
              placeholder="Password"
              disabled={attempt.isProcessing}
              mb={0}
              width="100%"
            />
          </Box>
          {auth2faType !== 'off' && (
            <Box mb={isRecoveryEnabled ? 2 : 3}>
              <Flex alignItems="flex-start">
                <FieldSelect
                  maxWidth="50%"
                  width="100%"
                  data-testid="mfa-select"
                  label="Multi-factor Type"
                  helperText={
                    isRecoveryEnabled && (
                      <ButtonLink
                        style={{ padding: '0px', minHeight: 0 }}
                        onClick={() => onRecover(false)}
                      >
                        Lost Two-Factor Device?
                      </ButtonLink>
                    )
                  }
                  value={mfaType}
                  options={mfaOptions}
                  onChange={opt => onSetMfaOption(opt as MfaOption, validator)}
                  mr={3}
                  mb={0}
                  isDisabled={isProcessing}
                  // Needed to prevent the menu from causing scroll bars to
                  // appear.
                  menuPosition="fixed"
                />
                {mfaType.value === 'otp' && (
                  <FieldInput
                    width="50%"
                    label="Authenticator Code"
                    rule={requiredToken}
                    autoComplete="one-time-code"
                    inputMode="numeric"
                    value={token}
                    onChange={e => setToken(e.target.value)}
                    placeholder="123 456"
                    disabled={attempt.isProcessing}
                    mb={0}
                  />
                )}
              </Flex>
            </Box>
          )}
          <ButtonPrimary
            width="100%"
            type="submit"
            size="extra-large"
            onClick={e => onLoginClick(e, validator)}
            disabled={isProcessing}
          >
            Sign In
          </ButtonPrimary>
        </Flex>
      )}
    </Validation>
  );
};

// Displays the primary login options and a list of secondary options.
const LoginOptions = ({
  next,
  refCallback,
  otherAuthTypes,
  showIdentifierFirstLogin,
  setShowIdentifierFirstLogin,
  ...otherProps
}: { otherAuthTypes: PrimaryAuthType[] } & Props & StepComponentProps) => {
  if (showIdentifierFirstLogin) {
    return (
      <Flex flexDirection="column" px={4} gap={3} ref={refCallback}>
        <FormIdentifierFirst
          onLoginWithSso={otherProps.onLoginWithSso}
          onUseLocalLogin={() => setShowIdentifierFirstLogin(false)}
        />
      </Flex>
    );
  }

  return (
    <Flex flexDirection="column" px={4} gap={3} ref={refCallback}>
      <AuthMethod
        {...otherProps}
        setShowIdentifierFirstLogin={setShowIdentifierFirstLogin}
        next={next}
        refCallback={refCallback}
        authType={otherProps.primaryAuthType}
        primary
        autoFocus
      />
      {otherAuthTypes.length > 0 && <Divider />}
      {otherAuthTypes.map(authType => (
        <AuthMethod
          key={authType}
          {...otherProps}
          setShowIdentifierFirstLogin={setShowIdentifierFirstLogin}
          next={next}
          refCallback={refCallback}
          authType={authType}
        />
      ))}
    </Flex>
  );
};

function AuthMethod({
  authType,
  primary,
  autoFocus,
  next,
  ...otherProps
}: {
  authType: PrimaryAuthType;
  primary?: boolean;
} & Props &
  StepComponentProps) {
  switch (authType) {
    case 'passwordless':
      return (
        <Passwordless {...otherProps} autoFocus={autoFocus} primary={primary} />
      );
    case 'sso':
      return <SsoList {...otherProps} autoFocus={autoFocus} />;
    case 'local':
      return primary ? (
        <LocalForm {...otherProps} autoFocus={true} />
      ) : (
        <Box py={2}>
          <ButtonSecondary size="extra-large" block onClick={next}>
            Sign in with Username and Password
          </ButtonSecondary>
        </Box>
      );
  }
}

// Displays a standalone local login form.
const LocalLogin = ({
  prev,
  refCallback,
  ...otherProps
}: Props & StepComponentProps) => {
  return (
    <Box px={4} ref={refCallback}>
      <LocalForm {...otherProps} autoFocus={true} />
      <Box pt={3} textAlign="center">
        <ButtonText
          width="100%"
          size="extra-large"
          disabled={otherProps.attempt.isProcessing}
          onClick={() => {
            otherProps.clearAttempt();
            prev();
          }}
        >
          Back
        </ButtonText>
      </Box>
    </Box>
  );
};

const Divider = () => (
  <Flex
    alignItems="center"
    justifyContent="center"
    flexDirection="column"
    borderBottom={1}
    borderColor="text.muted"
    my={3}
  >
    <StyledOr>Or</StyledOr>
  </Flex>
);

const StyledOr = styled.div`
  background: ${props => props.theme.colors.levels.surface};
  display: flex;
  align-items: center;
  font-size: 10px;
  height: 32px;
  width: 32px;
  justify-content: center;
  position: absolute;
  text-transform: uppercase;
`;

const loginViews = { default: [LoginOptions, LocalLogin] };

export type Props = {
  // Deprecated. TODO(bl-nero): Remove after e/ is updated.
  title?: string;
  isLocalAuthEnabled?: boolean;
  isPasswordlessEnabled: boolean;
  authProviders?: AuthProvider[];
  auth2faType?: Auth2faType;
  primaryAuthType: PrimaryAuthType;
  preferredMfaType?: PreferredMfaType;
  attempt: AttemptState;
  isRecoveryEnabled?: boolean;
  onRecover?: (isRecoverPassword: boolean) => void;
  clearAttempt?: () => void;
  onLoginWithSso(provider: AuthProvider): void;
  onLoginWithWebauthn(creds?: UserCredentials): void;
  onLogin(username: string, password: string, token: string): void;
  autoFocus?: boolean;
  showIdentifierFirstLogin?: boolean;
  setShowIdentifierFirstLogin?: (value: boolean) => void;
};

type AttemptState = ReturnType<typeof useAttempt>[0];
