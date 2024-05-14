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

import { within } from '@testing-library/react';
import { userEvent, UserEvent } from '@testing-library/user-event';

import TeleportContext from 'teleport/teleportContext';
import { ContextProvider } from 'teleport';
import MfaService from 'teleport/services/mfa';
import auth, { DeviceUsage } from 'teleport/services/auth';

import { AddAuthDeviceWizard } from '.';

const dummyCredential: Credential = { id: 'cred-id', type: 'public-key' };
let ctx: TeleportContext;
let user: UserEvent;
let onSuccess: jest.Mock;

beforeEach(() => {
  ctx = new TeleportContext();
  user = userEvent.setup();
  onSuccess = jest.fn();

  jest
    .spyOn(auth, 'createNewWebAuthnDevice')
    .mockResolvedValueOnce(dummyCredential);
  jest
    .spyOn(MfaService.prototype, 'saveNewWebAuthnDevice')
    .mockResolvedValueOnce(undefined);
  jest
    .spyOn(auth, 'createPrivilegeTokenWithWebauthn')
    .mockResolvedValueOnce('webauthn-privilege-token');
  jest
    .spyOn(auth, 'createPrivilegeTokenWithTotp')
    .mockImplementationOnce(token =>
      Promise.resolve(`totp-privilege-token-${token}`)
    );
  jest.spyOn(auth, 'createMfaRegistrationChallenge').mockResolvedValueOnce({
    qrCode: 'dummy-qr-code',
    webauthnPublicKey: {} as PublicKeyCredentialCreationOptions,
  });
  jest
    .spyOn(MfaService.prototype, 'addNewTotpDevice')
    .mockResolvedValueOnce(undefined);
});

afterEach(jest.resetAllMocks);

function TestWizard({
  privilegeToken,
  usage,
}: {
  privilegeToken?: string;
  usage: DeviceUsage;
}) {
  return (
    <ContextProvider ctx={ctx}>
      <AddAuthDeviceWizard
        usage={usage}
        auth2faType="optional"
        privilegeToken={privilegeToken}
        onClose={() => {}}
        onSuccess={onSuccess}
      />
    </ContextProvider>
  );
}

describe('flow without reauthentication', () => {
  test('adds a passkey', async () => {
    render(
      <TestWizard usage="passwordless" privilegeToken="privilege-token" />
    );

    const createStep = within(screen.getByTestId('create-step'));
    await user.click(
      createStep.getByRole('button', { name: 'Create a passkey' })
    );
    expect(auth.createNewWebAuthnDevice).toHaveBeenCalledWith({
      tokenId: 'privilege-token',
      deviceUsage: 'passwordless',
    });

    const saveStep = within(screen.getByTestId('save-step'));
    await user.type(saveStep.getByLabelText('Passkey Nickname'), 'new-passkey');
    await user.click(
      saveStep.getByRole('button', { name: 'Save the Passkey' })
    );
    expect(ctx.mfaService.saveNewWebAuthnDevice).toHaveBeenCalledWith({
      credential: dummyCredential,
      addRequest: {
        deviceName: 'new-passkey',
        deviceUsage: 'passwordless',
        tokenId: 'privilege-token',
      },
    });
    expect(onSuccess).toHaveBeenCalled();
  });

  test('adds a WebAuthn MFA', async () => {
    render(<TestWizard usage="mfa" privilegeToken="privilege-token" />);

    const createStep = within(screen.getByTestId('create-step'));
    await user.click(createStep.getByLabelText('Hardware Device'));
    await user.click(
      createStep.getByRole('button', { name: 'Create an MFA method' })
    );
    expect(auth.createNewWebAuthnDevice).toHaveBeenCalledWith({
      tokenId: 'privilege-token',
      deviceUsage: 'mfa',
    });

    const saveStep = within(screen.getByTestId('save-step'));
    await user.type(saveStep.getByLabelText('MFA Method Name'), 'new-mfa');
    await user.click(
      saveStep.getByRole('button', { name: 'Save the MFA method' })
    );
    expect(ctx.mfaService.saveNewWebAuthnDevice).toHaveBeenCalledWith({
      credential: dummyCredential,
      addRequest: {
        deviceName: 'new-mfa',
        deviceUsage: 'mfa',
        tokenId: 'privilege-token',
      },
    });
    expect(onSuccess).toHaveBeenCalled();
  });

  test('adds an authenticator app', async () => {
    render(<TestWizard usage="mfa" privilegeToken="privilege-token" />);

    const createStep = within(screen.getByTestId('create-step'));
    await user.click(createStep.getByLabelText('Authenticator App'));
    expect(createStep.getByRole('img')).toHaveAttribute(
      'src',
      'data:image/png;base64,dummy-qr-code'
    );
    await user.click(
      createStep.getByRole('button', { name: 'Create an MFA method' })
    );

    const saveStep = within(screen.getByTestId('save-step'));
    await user.type(saveStep.getByLabelText('MFA Method Name'), 'new-mfa');
    await user.type(saveStep.getByLabelText(/Authenticator Code/), '345678');
    await user.click(
      saveStep.getByRole('button', { name: 'Save the MFA method' })
    );
    expect(ctx.mfaService.addNewTotpDevice).toHaveBeenCalledWith({
      tokenId: 'privilege-token',
      secondFactorToken: '345678',
      deviceName: 'new-mfa',
    });
    expect(onSuccess).toHaveBeenCalled();
  });
});

describe('flow with reauthentication', () => {
  test('adds a passkey with WebAuthn reauthentication', async () => {
    render(<TestWizard usage="passwordless" />);

    const reauthenticateStep = within(
      screen.getByTestId('reauthenticate-step')
    );
    await user.click(reauthenticateStep.getByText('Verify my identity'));

    const createStep = within(screen.getByTestId('create-step'));
    await user.click(
      createStep.getByRole('button', { name: 'Create a passkey' })
    );
    expect(auth.createNewWebAuthnDevice).toHaveBeenCalledWith({
      tokenId: 'webauthn-privilege-token',
      deviceUsage: 'passwordless',
    });

    const saveStep = within(screen.getByTestId('save-step'));
    await user.type(saveStep.getByLabelText('Passkey Nickname'), 'new-passkey');
    await user.click(
      saveStep.getByRole('button', { name: 'Save the Passkey' })
    );
    expect(ctx.mfaService.saveNewWebAuthnDevice).toHaveBeenCalledWith({
      credential: dummyCredential,
      addRequest: {
        deviceName: 'new-passkey',
        deviceUsage: 'passwordless',
        tokenId: 'webauthn-privilege-token',
      },
    });
    expect(onSuccess).toHaveBeenCalled();
  });

  test('adds a passkey with OTP reauthentication', async () => {
    render(<TestWizard usage="passwordless" />);

    const reauthenticateStep = within(
      screen.getByTestId('reauthenticate-step')
    );
    await user.click(reauthenticateStep.getByText('Authenticator App'));
    await user.type(
      reauthenticateStep.getByLabelText('Authenticator Code'),
      '654987'
    );
    await user.click(reauthenticateStep.getByText('Verify my identity'));

    const createStep = within(screen.getByTestId('create-step'));
    await user.click(
      createStep.getByRole('button', { name: 'Create a passkey' })
    );
    expect(auth.createNewWebAuthnDevice).toHaveBeenCalledWith({
      tokenId: 'totp-privilege-token-654987',
      deviceUsage: 'passwordless',
    });

    const saveStep = within(screen.getByTestId('save-step'));
    await user.type(saveStep.getByLabelText('Passkey Nickname'), 'new-passkey');
    await user.click(
      saveStep.getByRole('button', { name: 'Save the Passkey' })
    );
    expect(ctx.mfaService.saveNewWebAuthnDevice).toHaveBeenCalledWith({
      credential: dummyCredential,
      addRequest: {
        deviceName: 'new-passkey',
        deviceUsage: 'passwordless',
        tokenId: 'totp-privilege-token-654987',
      },
    });
    expect(onSuccess).toHaveBeenCalled();
  });
});
