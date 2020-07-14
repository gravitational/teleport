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
import FormLogin from './FormLogin';
import { render, fireEvent } from 'design/utils/testing';

test('login with auth2faType: disabled', () => {
  const onLogin = jest.fn();
  const onLoginWithSso = jest.fn();
  const onLoginWithU2f = jest.fn();

  const { getByText, getByPlaceholderText } = render(
    <FormLogin
      title="titleText"
      auth2faType="off"
      authProviders={[]}
      attempt={{
        isFailed: false,
        isProcessing: false,
        message: '',
        isSuccess: undefined,
      }}
      onLogin={onLogin}
      onLoginWithSso={onLoginWithSso}
      onLoginWithU2f={onLoginWithU2f}
    />
  );

  fireEvent.change(getByPlaceholderText(/user name/i), {
    target: { value: 'username' },
  });
  fireEvent.change(getByPlaceholderText(/password/i), {
    target: { value: '123' },
  });

  fireEvent.click(getByText(/login/i));

  expect(onLogin).toHaveBeenCalledWith('username', '123', '');
  expect(onLoginWithSso).not.toHaveBeenCalled();
  expect(onLoginWithU2f).not.toHaveBeenCalled();
});

test('login with auth2faType: OTP', () => {
  const onLogin = jest.fn();
  const onLoginWithSso = jest.fn();
  const onLoginWithU2f = jest.fn();

  const { getByText, getByPlaceholderText } = render(
    <FormLogin
      auth2faType="otp"
      authProviders={[]}
      attempt={{
        isFailed: false,
        isProcessing: false,
        message: '',
        isSuccess: undefined,
      }}
      onLogin={onLogin}
      onLoginWithSso={onLoginWithSso}
      onLoginWithU2f={onLoginWithU2f}
    />
  );

  // fill form
  fireEvent.change(getByPlaceholderText(/user name/i), {
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
  expect(onLoginWithSso).not.toHaveBeenCalled();
  expect(onLoginWithU2f).not.toHaveBeenCalled();
});

test('login with auth2faType: U2F', () => {
  const onLogin = jest.fn();
  const onLoginWithSso = jest.fn();
  const onLoginWithU2f = jest.fn();

  const { getByText } = render(
    <FormLogin
      title="titleText"
      auth2faType="u2f"
      authProviders={[]}
      attempt={{
        isFailed: false,
        isProcessing: true,
        message: '',
        isSuccess: undefined,
      }}
      onLogin={onLogin}
      onLoginWithSso={onLoginWithSso}
      onLoginWithU2f={onLoginWithU2f}
    />
  );

  const expEl = getByText(
    /insert your U2F key and press the button on the key/i
  );

  expect(expEl).toBeInTheDocument();
  expect(getByText(/login/i)).toBeDisabled();
});

test('input validation error handling', () => {
  const onLogin = jest.fn();
  const onLoginWithSso = jest.fn();
  const onLoginWithU2f = jest.fn();

  const { getByText } = render(
    <FormLogin
      title="titleText"
      auth2faType="otp"
      attempt={{
        isFailed: false,
        isProcessing: false,
        message: '',
        isSuccess: undefined,
      }}
      onLogin={onLogin}
      onLoginWithSso={onLoginWithSso}
      onLoginWithU2f={onLoginWithU2f}
      authProviders={[]}
    />
  );

  fireEvent.click(getByText(/login/i));

  expect(onLogin).not.toHaveBeenCalled();
  expect(onLoginWithSso).not.toHaveBeenCalled();
  expect(onLoginWithU2f).not.toHaveBeenCalled();

  expect(getByText(/username is required/i)).toBeInTheDocument();
  expect(getByText(/password is required/i)).toBeInTheDocument();
  expect(getByText(/token is required/i)).toBeInTheDocument();
});

test('error handling', () => {
  const { getByText } = render(
    <FormLogin
      auth2faType="off"
      authProviders={[]}
      attempt={{
        isFailed: true,
        isProcessing: false,
        message: 'errMsg',
        isSuccess: undefined,
      }}
      onLogin={jest.fn()}
      onLoginWithSso={jest.fn()}
      onLoginWithU2f={jest.fn()}
    />
  );

  expect(getByText('errMsg')).toBeInTheDocument();
});

test('login with SSO providers', () => {
  const onLogin = jest.fn();
  const onLoginWithSso = jest.fn();
  const onLoginWithU2f = jest.fn();

  const { getByText } = render(
    <FormLogin
      auth2faType="off"
      authProviders={[
        { name: 'github', type: 'github', url: '' },
        { name: 'google', type: 'saml', url: '' },
      ]}
      attempt={{
        isFailed: false,
        isProcessing: false,
        message: '',
        isSuccess: undefined,
      }}
      onLogin={onLogin}
      onLoginWithSso={onLoginWithSso}
      onLoginWithU2f={onLoginWithU2f}
    />
  );

  fireEvent.click(getByText(/github/i));
  expect(onLoginWithSso).toHaveBeenCalledTimes(1);
  expect(onLogin).not.toHaveBeenCalled();
  expect(onLoginWithU2f).not.toHaveBeenCalled();
});
