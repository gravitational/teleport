/**
 * Copyright 2020 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import FormLogin, { Props } from './FormLogin';
import { render, fireEvent, wait } from 'design/utils/testing';

test('auth2faType: off', () => {
  const onLogin = jest.fn();

  const { getByText, getByPlaceholderText, queryByTestId } = render(
    <FormLogin {...props} onLogin={onLogin} />
  );

  // Rendering of mfa dropdown.
  expect(queryByTestId('mfa-select')).toBeNull();

  fireEvent.change(getByPlaceholderText(/username/i), {
    target: { value: 'username' },
  });
  fireEvent.change(getByPlaceholderText(/password/i), {
    target: { value: '123' },
  });

  fireEvent.click(getByText(/login/i));

  expect(onLogin).toHaveBeenCalledWith('username', '123', '');
});

test('auth2faType: otp', () => {
  const onLogin = jest.fn();

  const { getByText, getByPlaceholderText, getByTestId } = render(
    <FormLogin {...props} auth2faType="otp" onLogin={onLogin} />
  );

  // Rendering of mfa dropdown.
  expect(getByTestId('mfa-select')).not.toBeEmpty();

  // fill form
  fireEvent.change(getByPlaceholderText(/username/i), {
    target: { value: 'username' },
  });
  fireEvent.change(getByPlaceholderText(/password/i), {
    target: { value: '123' },
  });
  fireEvent.change(getByPlaceholderText(/123 456/i), {
    target: { value: '456' },
  });
  fireEvent.click(getByText(/login/i));

  expect(onLogin).toHaveBeenCalledWith('username', '123', '456');
});

test('auth2faType: u2f', async () => {
  const onLoginWithU2f = jest.fn();

  const { getByText, getByPlaceholderText, rerender, getByTestId } = render(
    <FormLogin {...props} auth2faType="u2f" onLoginWithU2f={onLoginWithU2f} />
  );

  // Rendering of mfa dropdown.
  expect(getByTestId('mfa-select')).not.toBeEmpty();

  // fill form
  fireEvent.change(getByPlaceholderText(/username/i), {
    target: { value: 'username' },
  });
  fireEvent.change(getByPlaceholderText(/password/i), {
    target: { value: '123' },
  });

  fireEvent.click(getByText(/login/i));
  expect(onLoginWithU2f).toHaveBeenCalledWith('username', '123');

  // test u2f instructions
  rerender(
    <FormLogin
      {...props}
      auth2faType="u2f"
      onLoginWithU2f={onLoginWithU2f}
      attempt={{
        isFailed: false,
        isProcessing: true,
        message: '',
        isSuccess: false,
      }}
    />
  );
  const expEl = getByText(
    /insert your hardware key and press the button on the key/i
  );
  expect(expEl).toBeInTheDocument();
  expect(getByText(/login/i)).toBeDisabled();
});

test('auth2faType: webauthn', async () => {
  const onLoginWithWebauthn = jest.fn();

  const { getByText, getByPlaceholderText, getByTestId } = render(
    <FormLogin
      {...props}
      auth2faType="webauthn"
      onLoginWithWebauthn={onLoginWithWebauthn}
    />
  );

  // Rendering of mfa dropdown.
  expect(getByTestId('mfa-select')).not.toBeEmpty();

  // fill form
  fireEvent.change(getByPlaceholderText(/username/i), {
    target: { value: 'username' },
  });
  fireEvent.change(getByPlaceholderText(/password/i), {
    target: { value: '123' },
  });

  fireEvent.click(getByText(/login/i));
  expect(onLoginWithWebauthn).toHaveBeenCalledWith('username', '123');
});

test('input validation error handling', async () => {
  const onLogin = jest.fn();
  const onLoginWithSso = jest.fn();
  const onLoginWithU2f = jest.fn();
  const onLoginWithWebauthn = jest.fn();

  const { getByText } = render(
    <FormLogin
      {...props}
      auth2faType="otp"
      onLogin={onLogin}
      onLoginWithSso={onLoginWithSso}
      onLoginWithU2f={onLoginWithU2f}
      onLoginWithWebauthn={onLoginWithWebauthn}
    />
  );

  await wait(() => {
    fireEvent.click(getByText(/login/i));
  });

  expect(onLogin).not.toHaveBeenCalled();
  expect(onLoginWithSso).not.toHaveBeenCalled();
  expect(onLoginWithU2f).not.toHaveBeenCalled();
  expect(onLoginWithWebauthn).not.toHaveBeenCalled();

  expect(getByText(/username is required/i)).toBeInTheDocument();
  expect(getByText(/password is required/i)).toBeInTheDocument();
  expect(getByText(/token is required/i)).toBeInTheDocument();
});

test('error rendering', () => {
  const { getByText } = render(
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

  expect(getByText('errMsg')).toBeInTheDocument();
});

test('sso providers', () => {
  const onLoginWithSso = jest.fn();

  const { getByText } = render(
    <FormLogin
      {...props}
      authProviders={[
        { name: 'github', type: 'github', url: '' },
        { name: 'google', type: 'saml', url: '' },
      ]}
      onLoginWithSso={onLoginWithSso}
    />
  );

  fireEvent.click(getByText(/github/i));
  expect(onLoginWithSso).toHaveBeenCalledTimes(1);
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
  onLoginWithU2f: null,
  onLoginWithWebauthn: null,
};
