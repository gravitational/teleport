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

import { fireEvent, render, screen } from 'design/utils/testing';

import history from 'teleport/services/history';

import FormLogin, { Props } from './FormLogin';

beforeEach(() => {
  jest.restoreAllMocks();
  jest.spyOn(history, 'hasAccessChangedParam').mockImplementation(() => false);
});

test('primary username and password with mfa off', () => {
  const onLogin = jest.fn();

  render(<FormLogin {...props} onLogin={onLogin} />);

  // Test only user/pwd form was rendered.
  expect(screen.queryByTestId('userpassword')).toBeVisible();
  expect(screen.queryByTestId('mfa-select')).not.toBeInTheDocument();
  expect(screen.queryByTestId('sso-list')).not.toBeInTheDocument();
  expect(screen.queryByTestId('passwordless')).not.toBeInTheDocument();

  // Test correct fn was called.
  fireEvent.change(screen.getByPlaceholderText(/username/i), {
    target: { value: 'username' },
  });
  fireEvent.change(screen.getByPlaceholderText(/password/i), {
    target: { value: '123' },
  });

  fireEvent.click(screen.getByRole('button', { name: /sign in/i }));

  expect(onLogin).toHaveBeenCalledWith('username', '123', '');
});

test('auth2faType: otp', () => {
  const onLogin = jest.fn();

  render(<FormLogin {...props} auth2faType="otp" onLogin={onLogin} />);

  // Rendering of mfa dropdown.
  expect(screen.getByTestId('mfa-select')).not.toBeEmptyDOMElement();

  // fill form
  fireEvent.change(screen.getByPlaceholderText(/username/i), {
    target: { value: 'username' },
  });
  fireEvent.change(screen.getByPlaceholderText(/password/i), {
    target: { value: '123' },
  });
  fireEvent.change(screen.getByPlaceholderText(/123 456/i), {
    target: { value: '456' },
  });
  fireEvent.click(screen.getByRole('button', { name: /sign in/i }));

  expect(onLogin).toHaveBeenCalledWith('username', '123', '456');
});

test('auth2faType: webauthn', async () => {
  const onLoginWithWebauthn = jest.fn();

  render(
    <FormLogin
      {...props}
      auth2faType="webauthn"
      onLoginWithWebauthn={onLoginWithWebauthn}
    />
  );

  // Rendering of mfa dropdown.
  expect(screen.getByTestId('mfa-select')).not.toBeEmptyDOMElement();

  // fill form
  fireEvent.change(screen.getByPlaceholderText(/username/i), {
    target: { value: 'username' },
  });
  fireEvent.change(screen.getByPlaceholderText(/password/i), {
    target: { value: '123' },
  });

  fireEvent.click(screen.getByRole('button', { name: /sign in/i }));
  expect(onLoginWithWebauthn).toHaveBeenCalledWith({
    username: 'username',
    password: '123',
  });
});

test('input validation error handling', async () => {
  const onLogin = jest.fn();
  const onLoginWithSso = jest.fn();
  const onLoginWithWebauthn = jest.fn();

  render(
    <FormLogin
      {...props}
      auth2faType="otp"
      onLogin={onLogin}
      onLoginWithSso={onLoginWithSso}
      onLoginWithWebauthn={onLoginWithWebauthn}
    />
  );

  fireEvent.click(screen.getByRole('button', { name: /sign in/i }));

  expect(onLogin).not.toHaveBeenCalled();
  expect(onLoginWithSso).not.toHaveBeenCalled();
  expect(onLoginWithWebauthn).not.toHaveBeenCalled();

  expect(screen.getByText(/username is required/i)).toBeInTheDocument();
  expect(screen.getByText(/password is required/i)).toBeInTheDocument();
  expect(screen.getByText(/token is required/i)).toBeInTheDocument();
});

test('error rendering', () => {
  render(
    <FormLogin
      {...props}
      auth2faType="off"
      attempt={{
        isFailed: true,
        isProcessing: false,
        message: 'errMsg',
        isSuccess: false,
      }}
    />
  );

  expect(screen.getByText('errMsg')).toBeInTheDocument();
});

test('primary sso', () => {
  const onLoginWithSso = jest.fn();

  render(
    <FormLogin
      {...props}
      authProviders={[
        { name: 'github', type: 'github', url: '' },
        { name: 'google', type: 'saml', url: '' },
      ]}
      onLoginWithSso={onLoginWithSso}
      primaryAuthType="sso"
    />
  );

  // Test only sso form was rendered.
  expect(screen.queryByTestId('sso-list')).toBeVisible();
  expect(screen.queryByTestId('passwordless')).not.toBeInTheDocument();
  expect(screen.queryByTestId('userpassword')).not.toBeInTheDocument();

  // Test clicking calls the right fn.
  fireEvent.click(screen.getByText(/github/i));
  expect(onLoginWithSso).toHaveBeenCalledTimes(1);
});

test('primary passwordless', () => {
  const onLoginWithSso = jest.fn();

  render(
    <FormLogin
      {...props}
      onLoginWithSso={onLoginWithSso}
      primaryAuthType="passwordless"
    />
  );

  // Test only passwordless form was rendered.
  expect(screen.queryByTestId('passwordless')).toBeVisible();
  expect(screen.queryByTestId('sso-list')).not.toBeInTheDocument();
  expect(screen.queryByTestId('userpassword')).not.toBeInTheDocument();
});

test('focuses the username input', () => {
  render(<FormLogin {...props} isPasswordlessEnabled />);

  expect(screen.getByLabelText(/username/i)).toHaveFocus();
  expect(
    screen.getByRole('button', { name: /sign in with a passkey/i })
  ).not.toHaveFocus();
});

test('focuses the passkey button', () => {
  render(
    <FormLogin
      {...props}
      isPasswordlessEnabled
      primaryAuthType="passwordless"
      authProviders={[
        { name: 'github', type: 'github', url: '' },
        { name: 'google', type: 'saml', url: '' },
      ]}
    />
  );

  expect(
    screen.getByRole('button', { name: /sign in with a passkey/i })
  ).toHaveFocus();
  expect(screen.getByRole('button', { name: 'github' })).not.toHaveFocus();
});

test('focuses the first SSO button', () => {
  render(
    <FormLogin
      {...props}
      authProviders={[
        { name: 'github', type: 'github', url: '' },
        { name: 'google', type: 'saml', url: '' },
      ]}
      isPasswordlessEnabled
      primaryAuthType="sso"
    />
  );

  expect(screen.getByRole('button', { name: 'github' })).toHaveFocus();
  expect(
    screen.getByRole('button', { name: /sign in with a passkey/i })
  ).not.toHaveFocus();
});

const props: Props = {
  auth2faType: 'off',
  authProviders: [],
  attempt: {
    isFailed: false,
    isProcessing: false,
    message: '',
    isSuccess: false,
  },
  onLogin: null,
  onLoginWithSso: null,
  onLoginWithWebauthn: null,
  isPasswordlessEnabled: false,
  primaryAuthType: 'local',
};
