/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import Dialog from 'design/Dialog';

import { createTeleportContext } from 'teleport/mocks/contexts';
import { ContextProvider } from 'teleport';

import { MfaDevice } from 'teleport/services/mfa';

import {
  ChangePasswordStep,
  ReauthenticateStep,
  createReauthOptions,
} from './ChangePasswordWizard';

export default {
  title: 'teleport/Account/Manage Devices/Change Password Wizard',
  decorators: [
    Story => {
      const ctx = createTeleportContext();
      return (
        <ContextProvider ctx={ctx}>
          <Dialog
            open={true}
            dialogCss={() => ({ width: '650px', padding: 0 })}
          >
            <Story />
          </Dialog>
        </ContextProvider>
      );
    },
  ],
};

export function Reauthenticate() {
  return <ReauthenticateStep {...stepProps} />;
}

export function ChangePasswordWithPasswordlessVerification() {
  return <ChangePasswordStep {...stepProps} reauthMethod="passwordless" />;
}

export function ChangePasswordWithMfaDeviceVerification() {
  return <ChangePasswordStep {...stepProps} reauthMethod="mfaDevice" />;
}

export function ChangePasswordWithOtpVerification() {
  return <ChangePasswordStep {...stepProps} reauthMethod="otp" />;
}

const devices: MfaDevice[] = [
  {
    id: '1',
    description: 'Hardware Key',
    name: 'touch_id',
    registeredDate: new Date(1628799417000),
    lastUsedDate: new Date(1628799417000),
    type: 'webauthn',
    usage: 'passwordless',
  },
  {
    id: '2',
    description: 'Hardware Key',
    name: 'solokey',
    registeredDate: new Date(1623722252000),
    lastUsedDate: new Date(1623981452000),
    type: 'webauthn',
    usage: 'mfa',
  },
  {
    id: '3',
    description: 'Authenticator App',
    name: 'iPhone',
    registeredDate: new Date(1618711052000),
    lastUsedDate: new Date(1626472652000),
    type: 'totp',
    usage: 'mfa',
  },
];

const defaultReauthOptions = createReauthOptions('optional', true, devices);

const stepProps = {
  // StepComponentProps
  next() {},
  prev() {},
  hasTransitionEnded: true,
  stepIndex: 0,
  flowLength: 2,
  refCallback: () => {},

  // Other props
  reauthOptions: defaultReauthOptions,
  reauthMethod: defaultReauthOptions[0].value,
  credential: { id: '', type: '' },
  onReauthMethodChange() {},
  onAuthenticated() {},
  onClose() {},
  onSuccess() {},
};
