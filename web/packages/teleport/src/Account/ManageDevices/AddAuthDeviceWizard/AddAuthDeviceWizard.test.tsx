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

import { render, screen, userEvent } from 'design/utils/testing';
import React from 'react';

import { within } from '@testing-library/react';

import TeleportContext from 'teleport/teleportContext';
import { ContextProvider } from 'teleport';
import MfaService from 'teleport/services/mfa';
import cfg from 'teleport/config';
import auth from 'teleport/services/auth/auth';

import { AddAuthDeviceWizard } from '.';

const dummyCredential: Credential = { id: 'cred-id', type: 'public-key' };

beforeEach(() => {
  jest.replaceProperty(cfg.auth, 'second_factor', 'optional');
  jest
    .spyOn(MfaService.prototype, 'createNewWebAuthnDevice')
    .mockResolvedValueOnce(dummyCredential);
  jest
    .spyOn(MfaService.prototype, 'saveNewWebAuthnDevice')
    .mockResolvedValueOnce('some-credential');
  jest
    .spyOn(auth, 'createPrivilegeTokenWithWebauthn')
    .mockResolvedValueOnce('webauthn-privilege-token');
  jest
    .spyOn(auth, 'createPrivilegeTokenWithTotp')
    .mockImplementationOnce(token =>
      Promise.resolve(`totp-privilege-token-${token}`)
    );
});

afterEach(jest.resetAllMocks);

interface TestWizardProps {
  ctx: TeleportContext;
  privilegeToken?: string;
  onSuccess(): void;
}
function TestWizard({ ctx, privilegeToken, onSuccess }: TestWizardProps) {
  return (
    <ContextProvider ctx={ctx}>
      <AddAuthDeviceWizard
        privilegeToken={privilegeToken}
        onClose={() => {}}
        onSuccess={onSuccess}
      />
    </ContextProvider>
  );
}

describe('flow without reauthentication', () => {
  test('adds a passkey', async () => {
    const ctx = new TeleportContext();
    const user = userEvent.setup();
    const onSuccess = jest.fn();
    render(
      <TestWizard
        ctx={ctx}
        onSuccess={onSuccess}
        privilegeToken="privilege-token"
      />
    );

    const createStep = within(screen.getByTestId('create-step'));
    await user.click(
      createStep.getByRole('button', { name: 'Create a passkey' })
    );
    expect(ctx.mfaService.createNewWebAuthnDevice).toHaveBeenCalledWith({
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
});

describe('flow with reauthentication', () => {
  test('adds a passkey with WebAuthn reauthentication', async () => {
    const ctx = new TeleportContext();
    const user = userEvent.setup();
    const onSuccess = jest.fn();
    render(<TestWizard ctx={ctx} onSuccess={onSuccess} />);

    const reauthenticateStep = within(
      screen.getByTestId('reauthenticate-step')
    );
    await user.click(reauthenticateStep.getByText('Verify my identity'));

    const createStep = within(screen.getByTestId('create-step'));
    await user.click(
      createStep.getByRole('button', { name: 'Create a passkey' })
    );
    expect(ctx.mfaService.createNewWebAuthnDevice).toHaveBeenCalledWith({
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
    const ctx = new TeleportContext();
    const user = userEvent.setup();
    const onSuccess = jest.fn();
    render(<TestWizard ctx={ctx} onSuccess={onSuccess} />);

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
    expect(ctx.mfaService.createNewWebAuthnDevice).toHaveBeenCalledWith({
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
