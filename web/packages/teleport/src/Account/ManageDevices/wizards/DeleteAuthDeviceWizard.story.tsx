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

import { initialize, mswLoader } from 'msw-storybook-addon';

import { createTeleportContext } from 'teleport/mocks/contexts';
import { ContextProvider } from 'teleport/index';

import { MfaDevice } from 'teleport/services/mfa';

import {
  DeleteAuthDeviceWizardStepProps,
  DeleteDeviceStep,
} from './DeleteAuthDeviceWizard';
import { ReauthenticateStep } from './ReauthenticateStep';

export default {
  title: 'teleport/Account/Manage Devices/Delete Device Wizard',
  loaders: [mswLoader],
  decorators: [
    Story => {
      const ctx = createTeleportContext();
      return (
        <ContextProvider ctx={ctx}>
          <Dialog open={true} dialogCss={() => ({ width: '650px' })}>
            <Story />
          </Dialog>
        </ContextProvider>
      );
    },
  ],
};

initialize();

export function Reauthenticate() {
  return <ReauthenticateStep {...stepProps} stepIndex={0} />;
}

export function DeletePasskey() {
  return (
    <DeleteDeviceStep
      {...stepProps}
      stepIndex={1}
      deviceToDelete={dummyPasskey}
    />
  );
}

export function DeleteMFA() {
  return (
    <DeleteDeviceStep
      {...stepProps}
      stepIndex={1}
      deviceToDelete={dummyHardwareDevice}
    />
  );
}

const dummyHardwareDevice: MfaDevice = {
  id: '2',
  description: 'Hardware Key',
  name: 'solokey',
  registeredDate: new Date(1623722252),
  lastUsedDate: new Date(1623981452),
  type: 'webauthn',
  usage: 'mfa',
};

const dummyPasskey: MfaDevice = {
  id: '3',
  description: 'Hardware Key',
  name: 'TouchID',
  registeredDate: new Date(1623722252),
  lastUsedDate: new Date(1623981452),
  type: 'webauthn',
  usage: 'passwordless',
};

const stepProps: DeleteAuthDeviceWizardStepProps = {
  // StepComponentProps
  next: () => {},
  prev: () => {},
  hasTransitionEnded: true,
  stepIndex: 0,
  flowLength: 2,
  refCallback: () => {},

  // Other props
  devices: [dummyHardwareDevice, dummyPasskey],
  deviceToDelete: dummyPasskey,
  privilegeToken: 'privilege-token',
  auth2faType: 'optional',
  onAuthenticated: () => {},
  onClose: () => {},
  onSuccess: () => {},
};
