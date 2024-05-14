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

import { Auth2faType } from 'shared/services';

import Dialog from 'design/Dialog';

import { initialize, mswLoader } from 'msw-storybook-addon';

import { rest } from 'msw';

import { DeviceUsage } from 'teleport/services/auth';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { ContextProvider } from 'teleport/index';

import cfg from 'teleport/config';

import {
  CreateDeviceStep,
  ReauthenticateStep,
  SaveDeviceStep,
} from './AddAuthDeviceWizard';

export default {
  title: 'teleport/Account/Manage Devices/Add Device Wizard',
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
  return <ReauthenticateStep {...stepProps} />;
}

export function CreatePasskey() {
  return <CreateDeviceStep {...stepProps} usage="passwordless" />;
}

export function CreateMfaHardwareDevice() {
  return (
    <CreateDeviceStep {...stepProps} usage="mfa" newMfaDeviceType="webauthn" />
  );
}

export function CreateMfaAppQrCodeLoading() {
  return <CreateDeviceStep {...stepProps} usage="mfa" newMfaDeviceType="otp" />;
}
CreateMfaAppQrCodeLoading.parameters = {
  msw: {
    handlers: [
      rest.post(
        cfg.getMfaCreateRegistrationChallengeUrl('privilege-token'),
        (req, res, ctx) => res(ctx.delay('infinite'))
      ),
    ],
  },
};

export function CreateMfaAppQrCodeFailed() {
  return <CreateDeviceStep {...stepProps} usage="mfa" newMfaDeviceType="otp" />;
}
CreateMfaAppQrCodeFailed.parameters = {
  msw: {
    handlers: [
      rest.post(
        cfg.getMfaCreateRegistrationChallengeUrl('privilege-token'),
        (req, res, ctx) => res(ctx.status(500))
      ),
    ],
  },
};

const dummyQrCode =
  'iVBORw0KGgoAAAANSUhEUgAAAB0AAAAdAQMAAABsXfVMAAAABlBMVEUAAAD///+l2Z/dAAAAAnRSTlP//8i138cAAAAJcEhZcwAACxIAAAsSAdLdfvwAAABrSURBVAiZY/gPBAxoxAcxh3qG71fv1zN8iQ8EEReBRACQ+H4ZKPZBFCj7/3v9f4aPU9vqGX4kFtUzfG5mBLK2aNUz/PM3AsmqAk2RNQTquLYLqDdG/z/QlGAgES4CFLu4GygrXF2Pbi+IAADZqFQFAjXZWgAAAABJRU5ErkJggg==';

export function CreateMfaApp() {
  return <CreateDeviceStep {...stepProps} usage="mfa" newMfaDeviceType="otp" />;
}
CreateMfaApp.parameters = {
  msw: {
    handlers: [
      rest.post(
        cfg.getMfaCreateRegistrationChallengeUrl('privilege-token'),
        (req, res, ctx) => res(ctx.json({ totp: { qrCode: dummyQrCode } }))
      ),
    ],
  },
};

export function SavePasskey() {
  return <SaveDeviceStep {...stepProps} usage="passwordless" />;
}

export function SaveMfaHardwareDevice() {
  return (
    <SaveDeviceStep {...stepProps} usage="mfa" newMfaDeviceType="webauthn" />
  );
}

export function SaveMfaAuthenticatorApp() {
  return <SaveDeviceStep {...stepProps} usage="mfa" newMfaDeviceType="otp" />;
}

const stepProps = {
  // StepComponentProps
  next: () => {},
  prev: () => {},
  hasTransitionEnded: true,
  stepIndex: 0,
  flowLength: 1,
  refCallback: () => {},

  // Other props
  privilegeToken: 'privilege-token',
  usage: 'passwordless' as DeviceUsage,
  auth2faType: 'optional' as Auth2faType,
  credential: { id: 'cred-id', type: 'public-key' },
  newMfaDeviceType: 'webauthn' as Auth2faType,
  onNewMfaDeviceTypeChange: () => {},
  onDeviceCreated: () => {},
  onAuthenticated: () => {},
  onClose: () => {},
  onPasskeyCreated: () => {},
  onSuccess: () => {},
};
