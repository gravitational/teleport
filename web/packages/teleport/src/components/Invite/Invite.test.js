/*
Copyright 2020 Gravitational, Inc.

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
import { MemoryRouter, Route } from 'react-router';
import { screen, fireEvent, act, render } from 'design/utils/testing';
import auth from 'teleport/services/auth';
import cfg from 'teleport/config';
import Invite from './Invite';
import { Logger } from 'shared/libs/logger';

describe('teleport/components/Invite', () => {
  beforeEach(() => {
    jest.spyOn(Logger.prototype, 'log').mockImplementation();
    jest.spyOn(auth, 'fetchPasswordToken').mockImplementation(async () => ({
      user: 'sam',
    }));
  });

  afterEach(() => {
    jest.clearAllMocks();
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
    fireEvent.click(screen.getByText('Create Account'));

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
    fireEvent.click(screen.getByText('Create Account'));

    expect(auth.resetPassword).toHaveBeenCalledWith(
      '5182',
      'pwd_value',
      '2222'
    );
  });

  it('reset password with U2F', async () => {
    jest.spyOn(cfg, 'getAuth2faType').mockImplementation(() => 'u2f');
    jest
      .spyOn(auth, 'resetPassword')
      .mockImplementation(() => new Promise(() => null));
    jest
      .spyOn(auth, 'resetPasswordWithU2f')
      .mockImplementation(() => new Promise(() => null));

    await act(async () => renderInvite());

    // fill out input boxes and trigger submit
    const pwdField = screen.getByPlaceholderText('Password');
    const pwdConfirmField = screen.getByPlaceholderText('Confirm Password');
    fireEvent.change(pwdField, { target: { value: 'pwd_value' } });
    fireEvent.change(pwdConfirmField, { target: { value: 'pwd_value' } });
    fireEvent.click(screen.getByText('Create Account'));

    expect(auth.resetPassword).not.toHaveBeenCalled();
    expect(auth.resetPasswordWithU2f).toHaveBeenCalledWith('5182', 'pwd_value');
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
      fireEvent.click(screen.getByText('Create Account'));
      reject(new Error('server_error'));
    });

    expect(screen.getByText('server_error')).toBeDefined();
  });
});

function renderInvite(url = `/web/invite/5182`) {
  render(
    <MemoryRouter initialEntries={[url]}>
      <Route path={cfg.routes.userInvite}>
        <Invite />
      </Route>
    </MemoryRouter>
  );
}
