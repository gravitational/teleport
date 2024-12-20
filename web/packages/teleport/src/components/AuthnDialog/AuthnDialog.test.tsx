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
import { getMfaChallengeOptions, SsoChallenge } from 'teleport/services/mfa';

import AuthnDialog from './AuthnDialog';

const mockSsoChallenge: SsoChallenge = {
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
    const mfa = makeMockState({
      challenge: {
        ssoChallenge: mockSsoChallenge,
      },
      attempt: {
        status: 'processing',
        statusText: '',
        data: null,
      },
    });
    render(<AuthnDialog mfaState={mfa} onClose={mockOnCancel} />);

    expect(screen.getByText('Verify Your Identity')).toBeInTheDocument();
    expect(
      screen.getByText('Select the method below to verify your identity:')
    ).toBeInTheDocument();
    expect(screen.getByText('Okta')).toBeInTheDocument();
    expect(screen.getByTestId('close-dialog')).toBeInTheDocument();
  });

  test('renders multi option dialog', () => {
    const challenge = {
      ssoChallenge: mockSsoChallenge,
      webauthnPublicKey: {
        challenge: new ArrayBuffer(1),
      },
    };
    const mfa = makeMockState({
      options: getMfaChallengeOptions(challenge),
      challenge,
      attempt: {
        status: 'processing',
        statusText: '',
        data: null,
      },
    });
    render(<AuthnDialog mfaState={mfa} onClose={mockOnCancel} />);

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
    const mfa = makeMockState({
      challenge: {},
      attempt: {
        status: 'error',
        statusText: errorText,
        data: null,
        error: new Error(errorText),
      },
    });
    render(<AuthnDialog mfaState={mfa} onClose={mockOnCancel} />);

    expect(screen.getByTestId('danger-alert')).toBeInTheDocument();
    expect(screen.getByText(errorText)).toBeInTheDocument();
  });

  test('sso button renders with callback', async () => {
    const mfa = makeMockState({
      challenge: {
        ssoChallenge: mockSsoChallenge,
      },
      attempt: {
        status: 'processing',
        statusText: '',
        data: null,
      },
      submit: jest.fn(),
    });
    render(<AuthnDialog mfaState={mfa} onClose={mockOnCancel} />);
    const ssoButton = screen.getByText('Okta');
    fireEvent.click(ssoButton);
    expect(mfa.submit).toHaveBeenCalledTimes(1);
  });

  test('webauthn button renders with callback', async () => {
    const mfa = makeMockState({
      challenge: {
        webauthnPublicKey: { challenge: new ArrayBuffer(0) },
      },
      attempt: {
        status: 'processing',
        statusText: '',
        data: null,
      },
      submit: jest.fn(),
    });
    render(<AuthnDialog mfaState={mfa} onClose={mockOnCancel} />);
    const webauthn = screen.getByText('Passkey or MFA Device');
    fireEvent.click(webauthn);
    expect(mfa.submit).toHaveBeenCalledTimes(1);
  });
});
