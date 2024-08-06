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

import React, { useState, useMemo } from 'react';
import styled from 'styled-components';
import {
  Card,
  Text,
  Flex,
  ButtonLink,
  ButtonPrimary,
  Box,
  ButtonText,
} from 'design';
import { Key, ArrowForward } from 'design/Icon';
import * as Alerts from 'design/Alert';
import {
  AuthProvider,
  Auth2faType,
  PreferredMfaType,
  PrimaryAuthType,
} from 'shared/services';
import { useAttempt, useRefAutoFocus } from 'shared/hooks';
import Validation, { Validator } from 'shared/components/Validation';
import FieldInput from 'shared/components/FieldInput';
import FieldSelect from 'shared/components/FieldSelect';
import {
  requiredToken,
  requiredField,
} from 'shared/components/Validation/rules';
import createMfaOptions, { MfaOption } from 'shared/utils/createMfaOptions';
import { StepSlider, StepComponentProps } from 'design/StepSlider';

import { UserCredentials } from 'teleport/services/auth';

import SSOButtonList from './SsoButtons';

export default function LoginForm(props: Props) {
  const {
    title,
    attempt,
    isLocalAuthEnabled = true,
    authProviders = [],
  } = props;

  const ssoEnabled = authProviders?.length > 0;

  // If local auth was not enabled, disregard any primary auth type config
  // and display sso providers if any.
  if (!isLocalAuthEnabled && ssoEnabled) {
    return (
      <Card my="5" mx="auto" width="464px" pb={4}>
        <Text typography="h3" pt={4} textAlign="center">
          {title}
        </Text>
        {attempt.isFailed && (
          <Alerts.Danger m={5} mb={0}>
            {attempt.message}
          </Alerts.Danger>
        )}
        <SsoList {...props} autoFocus={true} hasTransitionEnded={true} />
      </Card>
    );
  }

  if (!isLocalAuthEnabled) {
    return (
      <Card my="5" mx="auto" width="464px" px={5} pb={4}>
        <Text typography="h3" pt={4} textAlign="center">
          {title}
        </Text>
        <Alerts.Danger my={5}>Login has not been enabled</Alerts.Danger>
        <Text mb={2} typography="paragraph2" width="100%">
          The ability to login has not been enabled. Please contact your system
          administrator for more information.
        </Text>
      </Card>
    );
  }

  // Everything below requires local auth to be enabled.
  return (
    <Card my="5" mx="auto" width={464} pb={4}>
      <Text typography="h3" pt={4} textAlign="center">
        {title}
      </Text>
      {attempt.isFailed && (
        <Alerts.Danger m={5} mb={0}>
          {attempt.message}
        </Alerts.Danger>
      )}
      <StepSlider<typeof loginViews>
        flows={loginViews}
        currFlow={'default'}
        {...props}
      />
    </Card>
  );
}

const SsoList = ({
  attempt,
  authProviders,
  onLoginWithSso,
  autoFocus = false,
  hasTransitionEnded,
}: Props & { hasTransitionEnded?: boolean }) => {
  const ref = useRefAutoFocus<HTMLInputElement>({
    shouldFocus: hasTransitionEnded && autoFocus,
  });
  const { isProcessing } = attempt;
  return (
    <SSOButtonList
      prefixText="Login with"
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
}: Props & { hasTransitionEnded: boolean }) => {
  const ref = useRefAutoFocus<HTMLInputElement>({
    shouldFocus: hasTransitionEnded && autoFocus,
  });
  return (
    <Box px={5} pt={2} data-testid="passwordless" pb={1}>
      <StyledPaswordlessBtn
        setRef={ref}
        mt={3}
        py={2}
        px={3}
        width="100%"
        onClick={() => onLoginWithWebauthn()}
        disabled={attempt.isProcessing}
      >
        <Flex alignItems="center" justifyContent="space-between">
          <Flex alignItems="center">
            <Key mr={3} size="medium" />
            <Box>
              <Text typography="h6">Passwordless</Text>
              <Text
                fontSize={1}
                color={
                  attempt.isProcessing ? 'text.disabled' : 'text.slightlyMuted'
                }
              >
                Follow the prompt from your browser
              </Text>
            </Box>
          </Flex>
          <ArrowForward size="medium" />
        </Flex>
      </StyledPaswordlessBtn>
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
          px="5"
          pt="3"
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
              value={pass}
              onChange={e => setPass(e.target.value)}
              type="password"
              placeholder="Password"
              disabled={attempt.isProcessing}
              mb={0}
              width="100%"
            />
            {isRecoveryEnabled && (
              <Box textAlign="right">
                <ButtonLink
                  style={{ padding: '0px', minHeight: 0 }}
                  onClick={() => onRecover(true)}
                >
                  Forgot Password?
                </ButtonLink>
              </Box>
            )}
          </Box>
          {auth2faType !== 'off' && (
            <Box mb={isRecoveryEnabled ? 2 : 3}>
              <Flex alignItems="flex-end">
                <FieldSelect
                  maxWidth="50%"
                  width="100%"
                  data-testid="mfa-select"
                  label="Two-factor Type"
                  value={mfaType}
                  options={mfaOptions}
                  onChange={opt => onSetMfaOption(opt as MfaOption, validator)}
                  mr={3}
                  mb={0}
                  isDisabled={isProcessing}
                  menuIsOpen={true}
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
              {isRecoveryEnabled && (
                <ButtonLink
                  style={{ padding: '0px', minHeight: 0 }}
                  onClick={() => onRecover(false)}
                >
                  Lost Two-Factor Device?
                </ButtonLink>
              )}
            </Box>
          )}
          <ButtonPrimary
            width="100%"
            mt={3}
            mb={1}
            type="submit"
            size="large"
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

// Primary determines which authentication type to display
// on initial render of the login form.
const Primary = ({
  next,
  refCallback,
  hasTransitionEnded,
  ...otherProps
}: Props & StepComponentProps) => {
  const ssoEnabled = otherProps.authProviders?.length > 0;
  let otherOptionsAvailable = true;
  let $primary;

  switch (otherProps.primaryAuthType) {
    case 'passwordless':
      $primary = (
        <Passwordless
          {...otherProps}
          autoFocus={true}
          hasTransitionEnded={hasTransitionEnded}
        />
      );
      break;
    case 'sso':
      $primary = (
        <SsoList
          {...otherProps}
          autoFocus={true}
          hasTransitionEnded={hasTransitionEnded}
        />
      );
      break;
    case 'local':
      otherOptionsAvailable = otherProps.isPasswordlessEnabled || ssoEnabled;
      $primary = (
        <LocalForm
          {...otherProps}
          hasTransitionEnded={hasTransitionEnded}
          autoFocus={true}
        />
      );
      break;
  }

  return (
    <Box ref={refCallback}>
      {$primary}
      {otherOptionsAvailable && (
        <Box pt={3} mt={-1} textAlign="center">
          <ButtonText
            disabled={otherProps.attempt.isProcessing}
            onClick={() => {
              otherProps.clearAttempt();
              next();
            }}
          >
            Other sign-in options
          </ButtonText>
        </Box>
      )}
    </Box>
  );
};

// Secondary determines what other forms of authentication
// is allowed for the user to login with.
//
// There can be multiple authn types available, which will
// be visually separated by a divider.
const Secondary = ({
  prev,
  refCallback,
  ...otherProps
}: Props & StepComponentProps) => {
  const ssoEnabled = otherProps.authProviders?.length > 0;
  const { primaryAuthType, isPasswordlessEnabled } = otherProps;

  let $secondary;
  switch (primaryAuthType) {
    case 'passwordless':
      if (ssoEnabled) {
        $secondary = (
          <>
            <SsoList {...otherProps} autoFocus={true} />
            <Divider />
            <LocalForm {...otherProps} />
          </>
        );
      } else {
        $secondary = <LocalForm {...otherProps} autoFocus={true} />;
      }
      break;
    case 'sso':
      if (isPasswordlessEnabled) {
        $secondary = (
          <>
            <Passwordless {...otherProps} autoFocus={true} />
            <Divider />
            <LocalForm {...otherProps} />
          </>
        );
      } else {
        $secondary = <LocalForm {...otherProps} autoFocus={true} />;
      }
      break;
    case 'local':
      if (isPasswordlessEnabled) {
        $secondary = (
          <>
            <Passwordless {...otherProps} autoFocus={true} />
            {otherProps.isPasswordlessEnabled && ssoEnabled && <Divider />}
            {ssoEnabled && <SsoList {...otherProps} />}
          </>
        );
      } else {
        $secondary = <SsoList {...otherProps} autoFocus={true} />;
      }
      break;
  }
  return (
    <Box ref={refCallback}>
      {$secondary}
      <Box pt={3} textAlign="center">
        <ButtonText
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
    mx={5}
    mt={5}
    mb={2}
  >
    <StyledOr>Or</StyledOr>
  </Flex>
);

const StyledPaswordlessBtn = styled(ButtonText)`
  display: block;
  text-align: left;
  border: 1px solid ${({ theme }) => theme.colors.buttons.border.border};

  &:hover,
  &:focus {
    background: ${({ theme }) => theme.colors.buttons.border.hover};
    text-decoration: none;
  }

  &:active {
    background: ${({ theme }) => theme.colors.buttons.border.active};
  }

  &[disabled] {
    pointer-events: none;
    background: ${({ theme }) => theme.colors.buttons.bgDisabled};
  }
`;

const StyledOr = styled.div`
  background: ${props => props.theme.colors.levels.surface};
  display: flex;
  align-items: center;
  font-size: 10px;
  height: 32px;
  width: 32px;
  justify-content: center;
  position: absolute;
  z-index: 1;
`;

const loginViews = { default: [Primary, Secondary] };

export type Props = {
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
};

type AttemptState = ReturnType<typeof useAttempt>[0];
