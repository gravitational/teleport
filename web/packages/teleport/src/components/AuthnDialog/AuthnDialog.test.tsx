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

import { fireEvent, render, screen } from 'design/utils/testing';

import { makeDefaultMfaState, MfaState } from 'teleport/lib/useMfa';
import { SSOChallenge } from 'teleport/services/mfa';

import AuthnDialog from './AuthnDialog';

const mockSsoChallenge: SSOChallenge = {
  redirectUrl: 'url',
  requestId: '123',
  device: {
    displayName: 'Okta',
    connectorId: '123',
    connectorType: 'saml',
  },
  channelId: '123',
};

function makeMockState(partial: Partial<MfaState>): MfaState {
  const mfa = makeDefaultMfaState();
  return {
    ...mfa,
    ...partial,
  };
}

describe('AuthnDialog', () => {
  const mockOnCancel = jest.fn();

  beforeEach(() => {
    jest.clearAllMocks();
  });

  test('renders single option dialog', () => {
    const mfa = makeMockState({ ssoChallenge: mockSsoChallenge });
    render(<AuthnDialog mfa={mfa} onCancel={mockOnCancel} />);

    expect(screen.getByText('Verify Your Identity')).toBeInTheDocument();
    expect(
      screen.getByText('Select the method below to verify your identity:')
    ).toBeInTheDocument();
    expect(screen.getByText('Okta')).toBeInTheDocument();
    expect(screen.getByTestId('close-dialog')).toBeInTheDocument();
  });

  test('renders multi option dialog', () => {
    const mfa = makeMockState({
      ssoChallenge: mockSsoChallenge,
      webauthnPublicKey: {
        challenge: new ArrayBuffer(1),
      },
    });
    render(<AuthnDialog mfa={mfa} onCancel={mockOnCancel} />);

    expect(screen.getByText('Verify Your Identity')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Select one of the following methods to verify your identity:'
      )
    ).toBeInTheDocument();
    expect(screen.getByText('Okta')).toBeInTheDocument();
    expect(screen.getByTestId('close-dialog')).toBeInTheDocument();
  });

  test('displays error text when provided', () => {
    const errorText = 'Authentication failed';
    const mfa = makeMockState({ errorText });
    render(<AuthnDialog mfa={mfa} onCancel={mockOnCancel} />);

    expect(screen.getByTestId('danger-alert')).toBeInTheDocument();
    expect(screen.getByText(errorText)).toBeInTheDocument();
  });

  test('sso button renders with callback', async () => {
    const mfa = makeMockState({
      ssoChallenge: mockSsoChallenge,
      onSsoAuthenticate: jest.fn(),
    });
    render(<AuthnDialog mfa={mfa} onCancel={mockOnCancel} />);
    const ssoButton = screen.getByText('Okta');
    fireEvent.click(ssoButton);
    expect(mfa.onSsoAuthenticate).toHaveBeenCalledTimes(1);
  });

  test('webauthn button renders with callback', async () => {
    const mfa = makeMockState({
      webauthnPublicKey: { challenge: new ArrayBuffer(0) },
      onWebauthnAuthenticate: jest.fn(),
    });
    render(<AuthnDialog mfa={mfa} onCancel={mockOnCancel} />);
    const webauthn = screen.getByText('Passkey or MFA Device');
    fireEvent.click(webauthn);
    expect(mfa.onWebauthnAuthenticate).toHaveBeenCalledTimes(1);
  });
});
