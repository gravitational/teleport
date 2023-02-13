/*
Copyright 2021-2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { createMemoryRouter, RouterProvider } from 'react-router';
import { fireEvent, render, screen, waitFor } from 'design/utils/testing';
import { Logger } from 'shared/libs/logger';

import cfg from 'teleport/config';
import auth from 'teleport/services/auth';

import { userEventService } from 'teleport/services/userEvent';

import Invite from './Invite';
import Reset from './Reset';

const invitePath = '/web/invite/5182';
const inviteContinuePath = '/web/invite/5182/continue';
const resetPath = '/web/reset/5182';
const resetContinuePath = '/web/reset/5182/continue';

const routes = [
  {
    path: `${cfg.routes.userInvite}/*`,
    element: <Invite />,
  },
  {
    path: `${cfg.routes.userReset}/*`,
    element: <Reset />,
  },
];

describe('teleport/components/Welcome', () => {
  beforeEach(() => {
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
    const router = createMemoryRouter(routes, {
      initialEntries: [invitePath],
    });

    render(<RouterProvider router={router} />);

    expect(
      screen.getByText(/Please click the button below to create an account/i)
    ).toBeInTheDocument();

    expect(auth.fetchPasswordToken).not.toHaveBeenCalled();

    fireEvent.click(screen.getByText(/get started/i));

    expect(router.state.location.pathname).toBe(inviteContinuePath);

    await waitFor(() => {
      expect(auth.fetchPasswordToken).toHaveBeenCalled();
    });

    expect(screen.getByText(/confirm password/i)).toBeInTheDocument();
  });

  it('should have correct welcome prompt flow for reset', async () => {
    const router = createMemoryRouter(routes, {
      initialEntries: [resetPath],
    });

    render(<RouterProvider router={router} />);

    expect(
      screen.getByText(
        /Please click the button below to begin recovery of your account/i
      )
    ).toBeInTheDocument();

    expect(auth.fetchPasswordToken).not.toHaveBeenCalled();

    fireEvent.click(screen.getByText(/Continue/i));

    expect(router.state.location.pathname).toBe(resetContinuePath);

    await waitFor(() => {
      expect(auth.fetchPasswordToken).toHaveBeenCalled();
    });

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
    fireEvent.change(pwdField, { target: { value: 'pwd_value' } });
    fireEvent.change(pwdConfirmField, { target: { value: 'pwd_value' } });
    fireEvent.click(screen.getByRole('button'));

    expect(auth.resetPassword).toHaveBeenCalledWith({
      tokenId: '5182',
      password: 'pwd_value',
      otpCode: '',
      deviceName: '',
    });
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
    fireEvent.change(pwdField, { target: { value: 'pwd_value' } });
    fireEvent.change(pwdConfirmField, { target: { value: 'pwd_value' } });

    // Go to the next view.
    fireEvent.click(screen.getByText(/next/i));

    // Fill out otp code and trigger submit.
    const otpField = screen.getByPlaceholderText('123 456');
    fireEvent.change(otpField, { target: { value: '2222' } });
    fireEvent.click(screen.getByText(/submit/i));

    expect(auth.resetPassword).toHaveBeenCalledWith({
      tokenId: '5182',
      password: 'pwd_value',
      otpCode: '2222',
      deviceName: 'otp-device',
    });
  });

  it('reset password with webauthn', async () => {
    jest.spyOn(cfg, 'getAuth2faType').mockImplementation(() => 'webauthn');
    jest
      .spyOn(auth, 'resetPasswordWithWebauthn')
      .mockImplementation(() => new Promise(() => null));

    renderInvite();

    // Fill out password.
    const pwdField = await screen.findByPlaceholderText('Password');
    const pwdConfirmField = screen.getByPlaceholderText('Confirm Password');
    fireEvent.change(pwdField, { target: { value: 'pwd_value' } });
    fireEvent.change(pwdConfirmField, { target: { value: 'pwd_value' } });

    // Go to the next view.
    fireEvent.click(screen.getByText(/next/i));

    // Trigger submit.
    fireEvent.click(screen.getByText(/submit/i));

    expect(auth.resetPasswordWithWebauthn).toHaveBeenCalledWith({
      tokenId: '5182',
      password: 'pwd_value',
      deviceName: 'webauthn-device',
    });
  });

  it('reset password with passwordless', async () => {
    jest
      .spyOn(cfg, 'getPrimaryAuthType')
      .mockImplementation(() => 'passwordless');
    jest
      .spyOn(auth, 'resetPasswordWithWebauthn')
      .mockImplementation(() => new Promise(() => null));

    renderInvite();

    // Trigger submit.
    fireEvent.click(await screen.findByText(/submit/i));

    expect(auth.resetPasswordWithWebauthn).toHaveBeenCalledWith({
      tokenId: '5182',
      password: '',
      deviceName: 'passwordless-device',
    });
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
    fireEvent.change(pwdField, { target: { value: 'pwd_value' } });
    fireEvent.change(pwdConfirmField, { target: { value: 'pwd_value' } });
    fireEvent.click(screen.getByText(/next/i));

    // Default radio selection should be webauthn.
    expect(screen.getByDisplayValue('webauthn-device')).toBeInTheDocument();

    // Switch to otp.
    fireEvent.click(screen.getByText(/authenticator/i));
    expect(screen.getByDisplayValue('otp-device')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('123 456')).toBeInTheDocument();

    // Switch to none.
    fireEvent.click(screen.getByText(/none/i));
    expect(
      screen.queryByDisplayValue('webauthn-device')
    ).not.toBeInTheDocument();
    expect(screen.queryByDisplayValue('otp-device')).not.toBeInTheDocument();
  });
});

function renderInvite(url = inviteContinuePath) {
  const router = createMemoryRouter(routes, {
    initialEntries: [url],
  });

  render(<RouterProvider router={router} />);
}
