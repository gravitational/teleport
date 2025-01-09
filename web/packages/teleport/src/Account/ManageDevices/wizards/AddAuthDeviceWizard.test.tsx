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

import { AddAuthDeviceWizard } from '.';
import { AddAuthDeviceWizardStepProps } from './AddAuthDeviceWizard';

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
  jest.spyOn(auth, 'createMfaRegistrationChallenge').mockResolvedValueOnce({
    qrCode: 'dummy-qr-code',
    webauthnPublicKey: {} as PublicKeyCredentialCreationOptions,
  });
  jest
    .spyOn(MfaService.prototype, 'addNewTotpDevice')
    .mockResolvedValueOnce(undefined);
});

afterEach(jest.resetAllMocks);

function TestWizard(props: Partial<AddAuthDeviceWizardStepProps> = {}) {
  return (
    <ContextProvider ctx={ctx}>
      <AddAuthDeviceWizard
        usage="passwordless"
        auth2faType="on"
        onClose={() => {}}
        onSuccess={onSuccess}
        {...props}
      />
    </ContextProvider>
  );
}

describe('flow without reauthentication', () => {
  beforeEach(() => {
    jest.spyOn(auth, 'getMfaChallenge').mockResolvedValueOnce({});
    jest
      .spyOn(auth, 'createPrivilegeToken')
      .mockResolvedValueOnce('privilege-token');
  });

  test('adds a passkey', async () => {
    render(
      <TestWizard usage="passwordless" privilegeToken="privilege-token" />
    );

    await waitFor(() => {
      expect(screen.getByTestId('create-step')).toBeInTheDocument();
    });
    await user.click(screen.getByRole('button', { name: 'Create a passkey' }));
    expect(auth.createNewWebAuthnDevice).toHaveBeenCalledWith({
      tokenId: 'privilege-token',
      deviceUsage: 'passwordless',
    });

    expect(screen.getByTestId('save-step')).toBeInTheDocument();
    await user.type(screen.getByLabelText('Passkey Nickname'), 'new-passkey');
    await user.click(screen.getByRole('button', { name: 'Save the Passkey' }));
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

    await waitFor(() => {
      expect(screen.getByTestId('create-step')).toBeInTheDocument();
    });
    await user.click(screen.getByLabelText('Security Key'));
    await user.click(
      screen.getByRole('button', { name: 'Create an MFA method' })
    );
    expect(auth.createNewWebAuthnDevice).toHaveBeenCalledWith({
      tokenId: 'privilege-token',
      deviceUsage: 'mfa',
    });

    expect(screen.getByTestId('save-step')).toBeInTheDocument();
    await user.type(screen.getByLabelText('MFA Method Name'), 'new-mfa');
    await user.click(
      screen.getByRole('button', { name: 'Save the MFA method' })
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

    await waitFor(() => {
      expect(screen.getByTestId('create-step')).toBeInTheDocument();
    });

    await user.click(screen.getByLabelText('Authenticator App'));
    expect(screen.getByRole('img')).toHaveAttribute(
      'src',
      'data:image/png;base64,dummy-qr-code'
    );
    await user.click(
      screen.getByRole('button', { name: 'Create an MFA method' })
    );

    expect(screen.getByTestId('save-step')).toBeInTheDocument();
    await user.type(screen.getByLabelText('MFA Method Name'), 'new-mfa');
    await user.type(screen.getByLabelText(/Authenticator Code/), '345678');
    await user.click(
      screen.getByRole('button', { name: 'Save the MFA method' })
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
  const dummyMfaChallenge = {
    totpChallenge: true,
    webauthnPublicKey: {} as PublicKeyCredentialRequestOptions,
    ssoChallenge: {} as SsoChallenge,
  };

  beforeEach(() => {
    jest
      .spyOn(auth, 'getMfaChallenge')
      .mockResolvedValueOnce(dummyMfaChallenge);
    jest.spyOn(auth, 'getMfaChallengeResponse').mockResolvedValueOnce({});
    jest
      .spyOn(auth, 'createPrivilegeToken')
      .mockResolvedValueOnce('privilege-token');
  });

  test('adds a passkey with WebAuthn reauthentication', async () => {
    render(<TestWizard usage="passwordless" />);

    await waitFor(() => {
      expect(screen.getByTestId('reauthenticate-step')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Verify my identity'));

    await waitFor(() => {
      expect(screen.getByTestId('create-step')).toBeInTheDocument();
    });
    await user.click(screen.getByRole('button', { name: 'Create a passkey' }));
    expect(auth.getMfaChallengeResponse).toHaveBeenCalledWith(
      dummyMfaChallenge,
      'webauthn',
      ''
    );
    expect(auth.createNewWebAuthnDevice).toHaveBeenCalledWith({
      tokenId: 'privilege-token',
      deviceUsage: 'passwordless',
    });

    expect(screen.getByTestId('save-step')).toBeInTheDocument();
    await user.type(screen.getByLabelText('Passkey Nickname'), 'new-passkey');
    await user.click(screen.getByRole('button', { name: 'Save the Passkey' }));
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

  test('adds a passkey with OTP reauthentication', async () => {
    render(<TestWizard usage="passwordless" />);

    await waitFor(() => {
      expect(screen.getByTestId('reauthenticate-step')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Authenticator App'));
    await user.type(screen.getByLabelText('Authenticator Code'), '654987');
    await user.click(screen.getByText('Verify my identity'));

    await waitFor(() => {
      expect(screen.getByTestId('create-step')).toBeInTheDocument();
    });
    await user.click(screen.getByRole('button', { name: 'Create a passkey' }));
    expect(auth.getMfaChallengeResponse).toHaveBeenCalledWith(
      dummyMfaChallenge,
      'totp',
      '654987'
    );
    expect(auth.createNewWebAuthnDevice).toHaveBeenCalledWith({
      tokenId: 'privilege-token',
      deviceUsage: 'passwordless',
    });

    expect(screen.getByTestId('save-step')).toBeInTheDocument();
    await user.type(screen.getByLabelText('Passkey Nickname'), 'new-passkey');
    await user.click(screen.getByRole('button', { name: 'Save the Passkey' }));
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

  test('adds a passkey with SSO reauthentication', async () => {
    render(<TestWizard usage="passwordless" />);

    await waitFor(() => {
      expect(screen.getByTestId('reauthenticate-step')).toBeInTheDocument();
    });
    await user.click(screen.getByText('SSO'));
    await user.click(screen.getByText('Verify my identity'));

    expect(screen.getByTestId('create-step')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Create a passkey' }));
    expect(auth.getMfaChallengeResponse).toHaveBeenCalledWith(
      dummyMfaChallenge,
      'sso',
      ''
    );
    expect(auth.createNewWebAuthnDevice).toHaveBeenCalledWith({
      tokenId: 'privilege-token',
      deviceUsage: 'passwordless',
    });

    expect(screen.getByTestId('save-step')).toBeInTheDocument();
    await user.type(screen.getByLabelText('Passkey Nickname'), 'new-passkey');
    await user.click(screen.getByRole('button', { name: 'Save the Passkey' }));
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

  test('shows reauthentication options', async () => {
    render(<TestWizard usage="mfa" />);

    await waitFor(() => {
      expect(screen.getByTestId('reauthenticate-step')).toBeInTheDocument();
    });

    expect(screen.queryByLabelText(/passkey or security key/i)).toBeVisible();
    expect(screen.queryByLabelText(/authenticator app/i)).toBeVisible();
  });
});
