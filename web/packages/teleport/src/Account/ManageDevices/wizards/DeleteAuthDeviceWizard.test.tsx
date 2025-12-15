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

import { waitFor } from '@testing-library/react';
import { userEvent, UserEvent } from '@testing-library/user-event';

import { render, screen } from 'design/utils/testing';

import { ContextProvider } from 'teleport';
import auth from 'teleport/services/auth';
import MfaService, { SsoChallenge } from 'teleport/services/mfa';
import TeleportContext from 'teleport/teleportContext';

import { DeleteAuthDeviceWizard } from '.';
import { DeleteAuthDeviceWizardStepProps } from './DeleteAuthDeviceWizard';
import { dummyHardwareDevice, dummyPasskey } from './deviceCases';

let ctx: TeleportContext;
let user: UserEvent;
let onSuccess: jest.Mock;

const dummyMfaChallenge = {
  totpChallenge: true,
  webauthnPublicKey: {} as PublicKeyCredentialRequestOptions,
  ssoChallenge: {} as SsoChallenge,
};

beforeEach(() => {
  ctx = new TeleportContext();
  user = userEvent.setup();
  onSuccess = jest.fn();

  jest.spyOn(auth, 'getMfaChallenge').mockResolvedValueOnce(dummyMfaChallenge);
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

  await waitFor(() => {
    expect(screen.getByTestId('reauthenticate-step')).toBeInTheDocument();
  });
  await user.click(screen.getByText('Verify my identity'));

  expect(screen.getByTestId('delete-step')).toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: 'Delete' }));

  expect(auth.getMfaChallengeResponse).toHaveBeenCalledWith(
    dummyMfaChallenge,
    'webauthn',
    ''
  );
  expect(ctx.mfaService.removeDevice).toHaveBeenCalledWith(
    'privilege-token',
    'TouchID'
  );
  expect(onSuccess).toHaveBeenCalled();
});

test('deletes a device with OTP reauthentication', async () => {
  render(<TestWizard />);

  await waitFor(() => {
    expect(screen.getByTestId('reauthenticate-step')).toBeInTheDocument();
  });
  await user.click(screen.getByText('Authenticator App'));
  await user.type(screen.getByLabelText('Authenticator Code'), '654987');
  await user.click(screen.getByText('Verify my identity'));

  expect(screen.getByTestId('delete-step')).toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: 'Delete' }));

  expect(auth.getMfaChallengeResponse).toHaveBeenCalledWith(
    dummyMfaChallenge,
    'totp',
    '654987'
  );
  expect(ctx.mfaService.removeDevice).toHaveBeenCalledWith(
    'privilege-token',
    'TouchID'
  );
});

test('deletes a device with SSO reauthentication', async () => {
  render(<TestWizard />);

  await waitFor(() => {
    expect(screen.getByTestId('reauthenticate-step')).toBeInTheDocument();
  });
  await user.click(screen.getByText('SSO'));
  await user.click(screen.getByText('Verify my identity'));

  expect(screen.getByTestId('delete-step')).toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: 'Delete' }));

  expect(auth.getMfaChallengeResponse).toHaveBeenCalledWith(
    dummyMfaChallenge,
    'sso',
    ''
  );
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

    await waitFor(() => {
      expect(screen.getByTestId('reauthenticate-step')).toBeInTheDocument();
    });
    await user.click(screen.getByText('Verify my identity'));

    expect(screen.getByTestId('delete-step')).toBeInTheDocument();
    expect(screen.getByText(title)).toBeVisible();
    expect(screen.getByText(message)).toBeVisible();
  }
);
