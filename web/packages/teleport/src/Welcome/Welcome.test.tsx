/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
import { MemoryRouter, Route, Router } from 'react-router';
import { createMemoryHistory } from 'history';
import { fireEvent, render, screen, waitFor } from 'design/utils/testing';
import { Logger } from 'shared/libs/logger';

import { act } from '@testing-library/react';
import { userEvent, UserEvent } from '@testing-library/user-event';

import cfg from 'teleport/config';
import history from 'teleport/services/history';
import auth from 'teleport/services/auth';

import { userEventService } from 'teleport/services/userEvent';

import { NewCredentials } from 'teleport/Welcome/NewCredentials';

import { Welcome } from './Welcome';

const invitePath = '/web/invite/5182';
const inviteContinuePath = '/web/invite/5182/continue';
const resetPath = '/web/reset/5182';
const resetContinuePath = '/web/reset/5182/continue';

describe('teleport/components/Welcome', () => {
  let user: UserEvent;

  beforeEach(() => {
    user = userEvent.setup();
    jest.spyOn(Logger.prototype, 'log').mockImplementation();
    jest.spyOn(auth, 'fetchPasswordToken').mockImplementation(async () => ({
      user: 'sam',
      tokenId: 'test123',
      qrCode: 'test12345',
    }));
    jest
      .spyOn(userEventService, 'capturePreUserEvent')
      .mockImplementation(() => new Promise(() => null));
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  it('should have correct welcome prompt flow for invite', async () => {
    jest.spyOn(history, 'push').mockImplementation();

    const mockHistory = createMemoryHistory({
      initialEntries: [invitePath],
    });

    render(
      <Router history={mockHistory}>
        <Route path={cfg.routes.userInvite}>
          <Welcome NewCredentials={NewCredentials} />
        </Route>
      </Router>
    );

    expect(
      screen.getByText(/Please click the button below to create an account/i)
    ).toBeInTheDocument();

    expect(auth.fetchPasswordToken).not.toHaveBeenCalled();

    fireEvent.click(screen.getByText(/get started/i));
    act(() => mockHistory.push(inviteContinuePath));

    expect(history.push).toHaveBeenCalledWith(inviteContinuePath);
    await waitFor(() => {
      expect(auth.fetchPasswordToken).toHaveBeenCalled();
    });

    expect(screen.getByText(/confirm password/i)).toBeInTheDocument();
  });

  it('should have correct welcome prompt flow for reset', async () => {
    jest.spyOn(history, 'push').mockImplementation();

    const mockHistory = createMemoryHistory({
      initialEntries: [resetPath],
    });

    render(
      <Router history={mockHistory}>
        <Route path={cfg.routes.userReset}>
          <Welcome NewCredentials={NewCredentials} />
        </Route>
      </Router>
    );

    expect(
      screen.getByText(
        /Please click the button below to begin recovery of your account/i
      )
    ).toBeInTheDocument();

    expect(auth.fetchPasswordToken).not.toHaveBeenCalled();

    fireEvent.click(screen.getByText(/Continue/i));
    act(() => mockHistory.push(resetContinuePath));

    await waitFor(() => {
      expect(history.push).toHaveBeenCalledWith(resetContinuePath);
    });
    expect(auth.fetchPasswordToken).toHaveBeenCalled();

    expect(screen.getByText(/submit/i)).toBeInTheDocument();
  });

  it('reset password', async () => {
    jest.spyOn(cfg, 'getAuth2faType').mockImplementation(() => 'off');
    jest
      .spyOn(auth, 'resetPassword')
      .mockImplementation(() => new Promise(() => null));

    renderInvite();

    const pwdField = await screen.findByPlaceholderText('Password');
    const pwdConfirmField = screen.getByPlaceholderText('Confirm Password');

    // fill out input boxes and trigger submit
    fireEvent.change(pwdField, { target: { value: 'pwd_value_123' } });
    fireEvent.change(pwdConfirmField, { target: { value: 'pwd_value_123' } });
    fireEvent.click(screen.getByRole('button'));

    expect(auth.resetPassword).toHaveBeenCalledWith(
      expect.objectContaining({
        req: {
          tokenId: '5182',
          password: 'pwd_value_123',
          otpCode: '',
          deviceName: '',
        },
      })
    );
  });

  it('reset password with otp', async () => {
    jest.spyOn(cfg, 'getAuth2faType').mockImplementation(() => 'otp');
    jest
      .spyOn(auth, 'resetPassword')
      .mockImplementation(() => new Promise(() => null));

    renderInvite();

    // Fill out password.
    const pwdField = await screen.findByPlaceholderText('Password');
    const pwdConfirmField = screen.getByPlaceholderText('Confirm Password');
    fireEvent.change(pwdField, { target: { value: 'pwd_value_123' } });
    fireEvent.change(pwdConfirmField, { target: { value: 'pwd_value_123' } });

    // Go to the next view.
    fireEvent.click(screen.getByText(/next/i));

    // Fill out otp code and trigger submit.
    const otpField = screen.getByPlaceholderText('123 456');
    fireEvent.change(otpField, { target: { value: '2222' } });
    fireEvent.click(screen.getByText(/submit/i));

    expect(auth.resetPassword).toHaveBeenCalledWith(
      expect.objectContaining({
        req: {
          tokenId: '5182',
          password: 'pwd_value_123',
          otpCode: '2222',
          deviceName: 'otp-device',
        },
      })
    );
  });

  it('reset password with webauthn', async () => {
    jest.spyOn(cfg, 'getAuth2faType').mockImplementation(() => 'webauthn');
    jest
      .spyOn(auth, 'createNewWebAuthnDevice')
      .mockResolvedValueOnce({ id: 'dummy', type: 'public-key' });
    jest
      .spyOn(auth, 'resetPasswordWithWebauthn')
      .mockImplementation(() => new Promise(() => null));

    renderInvite();

    // Fill out password.
    const pwdField = await screen.findByPlaceholderText('Password');
    const pwdConfirmField = screen.getByPlaceholderText('Confirm Password');
    await user.type(pwdField, 'pwd_value_123');
    await user.type(pwdConfirmField, 'pwd_value_123');

    // Go to the next view.
    await user.click(screen.getByText(/next/i));

    // Create a WebAuthn credential.
    await user.click(screen.getByText(/create an MFA method/i));

    expect(auth.createNewWebAuthnDevice).toHaveBeenCalledWith({
      tokenId: '5182',
      deviceUsage: 'mfa',
    });

    // Trigger submit.
    await user.click(screen.getByText(/submit/i));

    expect(auth.resetPasswordWithWebauthn).toHaveBeenCalledWith(
      expect.objectContaining({
        req: {
          tokenId: '5182',
          password: 'pwd_value_123',
          deviceName: 'webauthn-device',
        },
      })
    );
  });

  it('reset password with passwordless', async () => {
    jest
      .spyOn(cfg, 'getPrimaryAuthType')
      .mockImplementation(() => 'passwordless');
    jest
      .spyOn(auth, 'createNewWebAuthnDevice')
      .mockResolvedValueOnce({ id: 'dummy', type: 'public-key' });
    jest
      .spyOn(auth, 'resetPasswordWithWebauthn')
      .mockImplementation(() => new Promise(() => null));

    renderInvite();

    // Create a WebAuthn credential.
    await user.click(await screen.findByText(/create a passkey/i));

    expect(auth.createNewWebAuthnDevice).toHaveBeenCalledWith({
      tokenId: '5182',
      deviceUsage: 'passwordless',
    });

    // Trigger submit.
    await user.click(await screen.findByText(/submit/i));

    expect(auth.resetPasswordWithWebauthn).toHaveBeenCalledWith(
      expect.objectContaining({
        req: {
          tokenId: '5182',
          password: '',
          deviceName: 'passwordless-device',
        },
      })
    );
  });

  it('switch between primary password to passwordless and vice versa', async () => {
    jest.spyOn(cfg, 'getPrimaryAuthType').mockImplementation(() => 'local');
    jest.spyOn(cfg, 'isPasswordlessEnabled').mockImplementation(() => true);

    renderInvite();

    // Switch to passwordless.
    fireEvent.click(await screen.findByText(/go passwordless/i));
    expect(screen.getByTestId('passwordless')).toBeVisible();

    // Switch back to password.
    fireEvent.click(screen.getByText(/back/i));
    expect(screen.getByTestId('password')).toBeVisible();
  });

  it('switch between primary passwordless to password and vice versa', async () => {
    jest
      .spyOn(cfg, 'getPrimaryAuthType')
      .mockImplementation(() => 'passwordless');

    renderInvite();

    // Switch to password.
    fireEvent.click(await screen.findByText(/use password/i));
    expect(screen.getByTestId('password')).toBeVisible();

    // Switch back to passwordless.
    fireEvent.click(screen.getByText(/back/i));
    expect(screen.getByTestId('passwordless')).toBeVisible();
  });

  it('switch between radio buttons when mfa is optional', async () => {
    jest.spyOn(cfg, 'getPrimaryAuthType').mockImplementation(() => 'local');
    jest.spyOn(cfg, 'getAuth2faType').mockImplementation(() => 'optional');

    renderInvite();

    // Fill out password to get to the next screen.
    const pwdField = await screen.findByPlaceholderText('Password');
    const pwdConfirmField = screen.getByPlaceholderText('Confirm Password');
    fireEvent.change(pwdField, { target: { value: 'pwd_value_123' } });
    fireEvent.change(pwdConfirmField, { target: { value: 'pwd_value_123' } });
    fireEvent.click(screen.getByText(/next/i));

    // Default radio selection should be webauthn.
    expect(
      screen.getByText(
        'You can use Touch ID, Face ID, Windows Hello, a hardware device, or an authenticator app as an MFA method.'
      )
    ).toBeInTheDocument();

    // Switch to otp.
    fireEvent.click(screen.getByRole('radio', { name: /authenticator app/i }));
    expect(screen.getByDisplayValue('otp-device')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('123 456')).toBeInTheDocument();

    // Switch to none.
    fireEvent.click(screen.getByRole('radio', { name: /none/i }));
    expect(
      screen.queryByDisplayValue('webauthn-device')
    ).not.toBeInTheDocument();
    expect(screen.queryByDisplayValue('otp-device')).not.toBeInTheDocument();
  });
});

function renderInvite(url = inviteContinuePath) {
  render(
    <MemoryRouter initialEntries={[url]}>
      <Route path={cfg.routes.userInviteContinue}>
        <Welcome NewCredentials={NewCredentials} />
      </Route>
    </MemoryRouter>
  );
}
