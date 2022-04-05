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
import { MemoryRouter, Route, Router } from 'react-router';
import { createMemoryHistory } from 'history';
import { screen, fireEvent, act, render, waitFor } from 'design/utils/testing';
import { Logger } from 'shared/libs/logger';
import cfg from 'teleport/config';
import history from 'teleport/services/history';
import auth from 'teleport/services/auth';
import Welcome from './Welcome';
import { AuthMfaOn, AuthMfaOptional } from './Welcome.story';

const invitePath = '/web/invite/5182';
const inviteContinuePath = '/web/invite/5182/continue';
const resetPath = '/web/reset/5182';
const resetContinuePath = '/web/reset/5182/continue';

describe('teleport/components/Welcome', () => {
  beforeEach(() => {
    jest.spyOn(Logger.prototype, 'log').mockImplementation();
    jest.spyOn(auth, 'fetchPasswordToken').mockImplementation(async () => ({
      user: 'sam',
      tokenId: 'test123',
      qrCode: 'test12345',
    }));
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
          <Welcome />
        </Route>
      </Router>
    );

    expect(
      screen.getByText(/Please click the button below to create an account/i)
    ).toBeInTheDocument();

    expect(auth.fetchPasswordToken).not.toHaveBeenCalled();

    await waitFor(() => {
      fireEvent.click(screen.getByText(/get started/i));
      mockHistory.push(inviteContinuePath);
    });

    expect(history.push).toHaveBeenCalledWith(inviteContinuePath);
    expect(auth.fetchPasswordToken).toHaveBeenCalled();

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
          <Welcome />
        </Route>
      </Router>
    );

    expect(
      screen.getByText(
        /Please click the button below to begin recovery of your account/i
      )
    ).toBeInTheDocument();

    expect(auth.fetchPasswordToken).not.toHaveBeenCalled();

    await waitFor(() => {
      fireEvent.click(screen.getByText(/Continue/i));
      mockHistory.push(resetContinuePath);
    });

    expect(history.push).toHaveBeenCalledWith(resetContinuePath);
    expect(auth.fetchPasswordToken).toHaveBeenCalled();

    expect(screen.getByText(/change password/i)).toBeInTheDocument();
  });

  it('reset password', async () => {
    jest.spyOn(cfg, 'getAuth2faType').mockImplementation(() => 'off');
    jest
      .spyOn(auth, 'resetPassword')
      .mockImplementation(() => new Promise(() => null));

    await act(async () => renderInvite());

    const pwdField = screen.getByPlaceholderText('Password');
    const pwdConfirmField = screen.getByPlaceholderText('Confirm Password');

    // fill out input boxes and trigger submit
    fireEvent.change(pwdField, { target: { value: 'pwd_value' } });
    fireEvent.change(pwdConfirmField, { target: { value: 'pwd_value' } });
    fireEvent.click(screen.getByRole('button'));

    expect(auth.resetPassword).toHaveBeenCalledWith('5182', 'pwd_value', '');
  });

  it('reset password with otp', async () => {
    jest.spyOn(cfg, 'getAuth2faType').mockImplementation(() => 'otp');
    jest
      .spyOn(auth, 'resetPassword')
      .mockImplementation(() => new Promise(() => null));

    await act(async () => renderInvite());

    const pwdField = screen.getByPlaceholderText('Password');
    const pwdConfirmField = screen.getByPlaceholderText('Confirm Password');
    const otpField = screen.getByPlaceholderText('123 456');

    // fill out input boxes and trigger submit
    fireEvent.change(pwdField, { target: { value: 'pwd_value' } });
    fireEvent.change(pwdConfirmField, { target: { value: 'pwd_value' } });
    fireEvent.change(otpField, { target: { value: '2222' } });
    fireEvent.click(screen.getByRole('button'));

    expect(auth.resetPassword).toHaveBeenCalledWith(
      '5182',
      'pwd_value',
      '2222'
    );
  });

  it('reset password with webauthn', async () => {
    jest.spyOn(cfg, 'getAuth2faType').mockImplementation(() => 'webauthn');
    jest
      .spyOn(auth, 'resetPassword')
      .mockImplementation(() => new Promise(() => null));
    jest
      .spyOn(auth, 'resetPasswordWithWebauthn')
      .mockImplementation(() => new Promise(() => null));

    await act(async () => renderInvite());

    // fill out input boxes and trigger submit
    const pwdField = screen.getByPlaceholderText('Password');
    const pwdConfirmField = screen.getByPlaceholderText('Confirm Password');
    fireEvent.change(pwdField, { target: { value: 'pwd_value' } });
    fireEvent.change(pwdConfirmField, { target: { value: 'pwd_value' } });
    fireEvent.click(screen.getByRole('button'));

    expect(auth.resetPasswordWithWebauthn).toHaveBeenCalledWith(
      '5182',
      'pwd_value'
    );
  });

  it('reset password error', async () => {
    let reject;
    jest.spyOn(cfg, 'getAuth2faType').mockImplementation(() => 'off');
    jest.spyOn(auth, 'resetPassword').mockImplementation(() => {
      return new Promise((resolve, _reject) => {
        reject = _reject;
      });
    });

    await act(async () => renderInvite());

    // fill out input boxes and trigger submit
    const pwdField = screen.getByPlaceholderText('Password');
    const pwdConfirmField = screen.getByPlaceholderText('Confirm Password');
    fireEvent.change(pwdField, { target: { value: 'pwd_value' } });
    fireEvent.change(pwdConfirmField, { target: { value: 'pwd_value' } });

    await act(async () => {
      fireEvent.click(screen.getByRole('button'));
      reject(new Error('server_error'));
    });

    expect(screen.getByText('server_error')).toBeDefined();
  });

  it('auth type "on" should render form with hardware key as first option in dropdown', () => {
    const { container } = render(
      <MemoryRouter initialEntries={[inviteContinuePath]}>
        <AuthMfaOn />
      </MemoryRouter>
    );
    expect(container).toMatchSnapshot();
  });

  it('auth type "optional" should render form with hardware key as first option in dropdown', () => {
    const { container } = render(
      <MemoryRouter initialEntries={[inviteContinuePath]}>
        <AuthMfaOptional />
      </MemoryRouter>
    );
    expect(container).toMatchSnapshot();
  });
});

function renderInvite(url = inviteContinuePath) {
  render(
    <MemoryRouter initialEntries={[url]}>
      <Route path={cfg.routes.userInviteContinue}>
        <Welcome />
      </Route>
    </MemoryRouter>
  );
}
