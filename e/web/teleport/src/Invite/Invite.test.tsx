import React from 'react';
import { MemoryRouter, Route } from 'react-router';
import { screen, fireEvent, act, render, wait } from 'design/utils/testing';
import cfg from 'teleport/config';
import history from 'teleport/services/history';
import auth from 'teleport/services/auth';
import Invite from './Invite';

test('should render recovery codes screen afterwards if the response includes them', async () => {
  jest.spyOn(history, 'push').mockImplementation();
  jest.spyOn(cfg, 'getAuth2faType').mockImplementation(() => 'otp');
  jest.spyOn(auth, 'fetchPasswordToken').mockImplementation(async () => ({
    user: 'sam',
    tokenId: 'test123',
    qrCode: 'test12345',
  }));

  jest
    .spyOn(auth, 'resetPassword')
    .mockResolvedValue([
      'tele-recovery-code-1',
      'tele-recovery-code-2',
      'tele-recovery-code-3',
    ]);
  await act(async () => renderInvite());

  const pwdField = screen.getByPlaceholderText('Password');
  const pwdConfirmField = screen.getByPlaceholderText('Confirm Password');
  const otpField = screen.getByPlaceholderText('123 456');

  // fill out input boxes and trigger submit
  fireEvent.change(pwdField, { target: { value: 'pwd_value' } });
  fireEvent.change(pwdConfirmField, { target: { value: 'pwd_value' } });
  fireEvent.change(otpField, { target: { value: '2222' } });

  await wait(() => fireEvent.click(screen.getByText('Create Account')));

  expect(auth.resetPassword).toHaveBeenCalledWith('5182', 'pwd_value', '2222');

  expect(screen.getByText('Backup & Recovery Codes')).toBeInTheDocument();
  expect(screen.getByText(/tele-recovery-code-1/i)).toBeInTheDocument();
  expect(screen.getByText(/tele-recovery-code-2/i)).toBeInTheDocument();
  expect(screen.getByText(/tele-recovery-code-3/i)).toBeInTheDocument();
});

test('should not render recovery codes screen afterwards if the response doesnt include them', async () => {
  jest.spyOn(cfg, 'getAuth2faType').mockImplementation(() => 'otp');
  jest.spyOn(auth, 'fetchPasswordToken').mockImplementation(async () => ({
    user: 'sam',
    tokenId: 'test123',
    qrCode: 'test12345',
  }));

  jest.spyOn(auth, 'resetPassword').mockResolvedValue(undefined);
  await act(async () => renderInvite());

  const pwdField = screen.getByPlaceholderText('Password');
  const pwdConfirmField = screen.getByPlaceholderText('Confirm Password');
  const otpField = screen.getByPlaceholderText('123 456');

  // fill out input boxes and trigger submit
  fireEvent.change(pwdField, { target: { value: 'pwd_value' } });
  fireEvent.change(pwdConfirmField, { target: { value: 'pwd_value' } });
  fireEvent.change(otpField, { target: { value: '2222' } });

  await wait(() => fireEvent.click(screen.getByText('Create Account')));

  expect(auth.resetPassword).toHaveBeenCalledWith('5182', 'pwd_value', '2222');

  expect(screen.queryByText('Backup & Recovery Codes')).not.toBeInTheDocument();
  expect(screen.queryByText(/tele-recovery-code-1/i)).not.toBeInTheDocument();
  expect(screen.queryByText(/tele-recovery-code-2/i)).not.toBeInTheDocument();
  expect(screen.queryByText(/tele-recovery-code-3/i)).not.toBeInTheDocument();
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
