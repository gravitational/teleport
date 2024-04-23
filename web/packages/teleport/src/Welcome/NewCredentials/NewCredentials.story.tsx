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

import React from 'react';

import { WelcomeWrapper } from 'design/Onboard/WelcomeWrapper';

import {
  NewMfaDevice,
  NewMfaDeviceProps,
} from 'teleport/Welcome/NewCredentials/NewMfaDevice';

import { NewCredentials } from './NewCredentials';
import { NewCredentialsProps } from './types';

export default {
  title: 'Teleport/Welcome/Form',
  component: NewCredentials,
};

export const PasswordOnly = () =>
  renderNewCredentials({ isPasswordlessEnabled: false });

export const PasswordOnlyError = () =>
  renderNewCredentials({
    isPasswordlessEnabled: false,
    submitAttempt: {
      status: 'failed',
      statusText: 'some server error message',
    },
  });

export const PrimaryPasswordNoMfa = () => renderNewCredentials({});

export const PrimaryPasswordWithMfa = () =>
  renderNewCredentials({ auth2faType: 'optional' });

export const PrimaryPasswordlessNoMfa = () =>
  renderNewCredentials({ primaryAuthType: 'passwordless' });

export const PrimaryPasswordlessPasskeyCreated = () =>
  renderNewCredentials({
    primaryAuthType: 'passwordless',
    credential: {
      id: 'some-credential',
      type: 'public-key',
    },
  });

export const PrimaryPasswordlessWithMfa = () =>
  renderNewCredentials({
    primaryAuthType: 'passwordless',
    auth2faType: 'optional',
  });

export const PrimaryPasswordlessError = () =>
  renderNewCredentials({
    primaryAuthType: 'passwordless',
    submitAttempt: {
      status: 'failed',
      statusText: 'some server error message',
    },
  });

export const MfaDeviceOtp = () =>
  renderMfaFlow({
    auth2faType: 'otp',
  });

export const MfaDeviceWebauthn = () =>
  renderMfaFlow({
    auth2faType: 'webauthn',
  });

export const MfaDeviceWebauthnKeyCreated = () =>
  renderMfaFlow({
    auth2faType: 'webauthn',
    credential: {
      id: 'some-credential',
      type: 'public-key',
    },
  });

export const MfaDeviceOptional = () =>
  renderMfaFlow({
    auth2faType: 'optional',
  });

export const MfaDeviceOn = () =>
  renderMfaFlow({
    auth2faType: 'on',
  });

export const MfaDeviceError = () =>
  renderMfaFlow({
    auth2faType: 'otp',
    submitAttempt: {
      status: 'failed',
      statusText: 'some server error message',
    },
  });

export const ExpiredInvite = () =>
  renderNewCredentials({
    fetchAttempt: { status: 'failed' },
  });

export const ExpiredReset = () =>
  renderNewCredentials({
    fetchAttempt: { status: 'failed' },
    resetMode: true,
  });

const recoveryCodes = {
  codes: [
    'tele-testword-testword-testword-testword-testword-testword-testword',
    'tele-testword-testword-testword-testword-testword-testword-testword-testword',
    'tele-testword-testword-testword-testword-testword-testword-testword',
  ],
  createdDate: new Date('2019-08-30T11:00:00.00Z'),
};

export const RecoveryCodesInvite = () =>
  renderNewCredentials({
    recoveryCodes: recoveryCodes,
  });

export const RecoveryCodesReset = () =>
  renderNewCredentials({
    recoveryCodes: recoveryCodes,
    resetMode: true,
  });

export const SuccessRegister = () =>
  renderNewCredentials({
    success: true,
  });

export const SuccessReset = () =>
  renderNewCredentials({
    success: true,
    resetMode: true,
  });

export const SuccessRegisterDashboard = () =>
  renderNewCredentials({
    success: true,
    isDashboard: true,
  });

export const SuccessResetDashboard = () =>
  renderNewCredentials({
    success: true,
    resetMode: true,
    isDashboard: true,
  });

const makeNewMfaDeviceProps = (
  overrides: Partial<NewMfaDeviceProps> = {}
): NewMfaDeviceProps => {
  return Object.assign(
    {
      ...makeNewCredProps(overrides),
      next: () => null,
      prev: () => null,
      changeFlow: () => null,
      refCallback: () => null,
      hasTransitionEnded: true,
      password: '',
      updatePassword: () => null,
      stepIndex: 0,
      flowLength: 1,
    },
    overrides
  );
};

const makeNewCredProps = (
  overrides: Partial<NewCredentialsProps> = {}
): NewCredentialsProps => {
  return Object.assign(
    {
      auth2faType: 'off',
      primaryAuthType: 'local',
      isPasswordlessEnabled: true,
      submitAttempt: { status: '' },
      clearSubmitAttempt: () => null,
      fetchAttempt: { status: 'success' },
      onSubmitWithWebauthn: () => null,
      createNewWebAuthnDevice: () => null,
      onSubmit: () => null,
      redirect: () => null,
      success: false,
      finishedRegister: () => null,
      recoveryCodes: null,
      resetToken: {
        user: 'john@example.com',
        tokenId: 'test123',
        qrCode:
          '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',
      },
      isDashboard: false,
    },
    overrides
  );
};

/**
 * Renders New Credentials
 *
 * @remarks
 * renderNewCredentials wraps the NewCredentials component in a WelcomeWrapper. Every instance of NewCredentials
 * is wrapped in a WelcomeWrapper via the Welcome parent component.
 *
 * @param partialProps - partial NewCredentialProps to override default values on individual stories
 *
 */
const renderNewCredentials = (partialProps: Partial<NewCredentialsProps>) => {
  const props = makeNewCredProps(partialProps);

  return (
    <WelcomeWrapper>
      <NewCredentials {...props} />
    </WelcomeWrapper>
  );
};

/**
 * Renders New MFA Device
 *
 * @remarks
 * renderMfaFlow wraps the NewMfaDevice component in a WelcomeWrapper. Every instance of NewMfaDevice
 * is wrapped in a WelcomeWrapper via the NewCredentials parent component.
 *
 * @param partialProps - partial NewCredentialProps to override default values on individual stories
 *
 */
const renderMfaFlow = (partialProps: Partial<NewCredentialsProps>) => {
  const props = makeNewMfaDeviceProps(partialProps);

  return (
    <WelcomeWrapper>
      <NewMfaDevice {...props} />
    </WelcomeWrapper>
  );
};
