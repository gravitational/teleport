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

import { render, screen } from 'design/utils/testing';
import React from 'react';

import { waitFor, within } from '@testing-library/react';
import { userEvent, UserEvent } from '@testing-library/user-event';

import TeleportContext from 'teleport/teleportContext';
import { ContextProvider } from 'teleport';
import MfaService from 'teleport/services/mfa';
import auth from 'teleport/services/auth';

import { DeleteAuthDeviceWizardStepProps } from './DeleteAuthDeviceWizard';

import { dummyPasskey, dummyHardwareDevice, deviceCases } from './deviceCases';

import { DeleteAuthDeviceWizard } from '.';

let ctx: TeleportContext;
let user: UserEvent;
let onSuccess: jest.Mock;

beforeEach(() => {
  ctx = new TeleportContext();
  user = userEvent.setup();
  onSuccess = jest.fn();

  jest.spyOn(auth, 'getMfaChallenge').mockResolvedValueOnce({
    totpChallenge: true,
    webauthnPublicKey: {} as PublicKeyCredentialRequestOptions,
  });
  jest.spyOn(auth, 'getMfaChallengeResponse').mockResolvedValueOnce({});
  jest
    .spyOn(auth, 'createPrivilegeToken')
    .mockResolvedValueOnce('privilege-token');
  jest
    .spyOn(MfaService.prototype, 'removeDevice')
    .mockResolvedValueOnce(undefined);
});

afterEach(jest.resetAllMocks);

function TestWizard(props: Partial<DeleteAuthDeviceWizardStepProps>) {
  return (
    <ContextProvider ctx={ctx}>
      <DeleteAuthDeviceWizard
        deviceToDelete={dummyPasskey}
        onClose={() => {}}
        onSuccess={onSuccess}
        {...props}
      />
    </ContextProvider>
  );
}

test('deletes a device with WebAuthn reauthentication', async () => {
  render(<TestWizard />);

  let reauthenticateStep;
  await waitFor(() => {
    reauthenticateStep = within(screen.getByTestId('reauthenticate-step'));
  });
  await user.click(reauthenticateStep.getByText('Verify my identity'));

  const deleteStep = within(screen.getByTestId('delete-step'));
  await user.click(deleteStep.getByRole('button', { name: 'Delete' }));

  expect(ctx.mfaService.removeDevice).toHaveBeenCalledWith(
    'privilege-token',
    'TouchID'
  );
  expect(onSuccess).toHaveBeenCalled();
});

test('deletes a device with OTP reauthentication', async () => {
  render(<TestWizard />);

  let reauthenticateStep;
  await waitFor(() => {
    reauthenticateStep = within(screen.getByTestId('reauthenticate-step'));
  });
  await user.click(reauthenticateStep.getByText('Authenticator App'));
  await user.type(
    reauthenticateStep.getByLabelText('Authenticator Code'),
    '654987'
  );
  await user.click(reauthenticateStep.getByText('Verify my identity'));

  const deleteStep = within(screen.getByTestId('delete-step'));
  await user.click(deleteStep.getByRole('button', { name: 'Delete' }));

  expect(ctx.mfaService.removeDevice).toHaveBeenCalledWith(
    'privilege-token',
    'TouchID'
  );
});

test.each([
  {
    case: 'a passkey',
    device: dummyPasskey,
    message: 'Are you sure you want to delete your "TouchID" passkey?',
    title: 'Delete Passkey',
  },
  {
    case: 'an MFA method',
    device: dummyHardwareDevice,
    message: 'Are you sure you want to delete your "solokey" MFA method?',
    title: 'Delete MFA Method',
  },
])(
  'shows correct title and message for $case',
  async ({ device, title, message }) => {
    render(<TestWizard deviceToDelete={device} />);

    const reauthenticateStep = await waitFor(() => {
      return within(screen.getByTestId('reauthenticate-step'));
    });
    await user.click(reauthenticateStep.getByText('Verify my identity'));

    const deleteStep = within(screen.getByTestId('delete-step'));
    expect(deleteStep.getByText(title)).toBeVisible();
    expect(deleteStep.getByText(message)).toBeVisible();
  }
);
