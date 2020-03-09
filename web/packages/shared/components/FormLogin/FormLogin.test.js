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
import { Auth2faTypeEnum, AuthProviderTypeEnum } from '../../services/enums';
import { TypeEnum as Type } from '../ButtonSso/';

test('login with auth2faType: disabled with ssoProviders', () => {
  const onLogin = jest.fn();
  const onLoginWithSso = jest.fn();
  const onLoginWithU2f = jest.fn();

  const { getByText, getByPlaceholderText } = render(
    <FormLogin
      title="titleText"
      auth2faType={Auth2faTypeEnum.DISABLED}
      authProviders={[
        { type: AuthProviderTypeEnum.OIDC, url: '', name: Type.GITHUB },
      ]}
      attempt={{ isFailed: false, isProcessing: false, message: '' }}
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
  fireEvent.click(getByText(/login/i));

  expect(onLogin).toHaveBeenCalledWith('username', '123', '');
  expect(onLoginWithSso).not.toHaveBeenCalled();
  expect(onLoginWithU2f).not.toHaveBeenCalled();
  jest.clearAllMocks();

  // test ssoProvider buttons
  fireEvent.click(getByText(/github/i));
  expect(onLoginWithSso).toHaveBeenCalledTimes(1);
  expect(onLogin).not.toHaveBeenCalled();
  expect(onLoginWithU2f).not.toHaveBeenCalled();
});

test('login with auth2faType: OTP', () => {
  const onLogin = jest.fn();
  const onLoginWithSso = jest.fn();
  const onLoginWithU2f = jest.fn();

  const { getByText, getByPlaceholderText } = render(
    <FormLogin
      auth2faType={Auth2faTypeEnum.OTP}
      authProviders={[]}
      attempt={{ isFailed: false, isProcessing: false, message: '' }}
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
      auth2faType={Auth2faTypeEnum.UTF}
      authProviders={[]}
      attempt={{ isFailed: false, isProcessing: true, message: '' }}
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

test('input validation errors with OTP', () => {
  const onLogin = jest.fn();
  const onLoginWithSso = jest.fn();
  const onLoginWithU2f = jest.fn();

  const { getByText } = render(
    <FormLogin
      title="titleText"
      auth2faType={Auth2faTypeEnum.OTP}
      attempt={{ isFailed: false, isProcessing: false, message: '' }}
      onLogin={onLogin}
      onLoginWithSso={onLoginWithSso}
      onLoginWithU2f={onLoginWithU2f}
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

test('attempt object with prop isFailing and error message', () => {
  const onLogin = jest.fn();
  const onLoginWithSso = jest.fn();
  const onLoginWithU2f = jest.fn();

  const { getByText } = render(
    <FormLogin
      auth2faType={Auth2faTypeEnum.DISABLED}
      authProviders={[]}
      attempt={{ isFailed: true, isProcessing: false, message: 'errMsg' }}
      onLogin={onLogin}
      onLoginWithSso={onLoginWithSso}
      onLoginWithU2f={onLoginWithU2f}
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
      authProviders={[
        { name: 'github', type: '', url: '' },
        { name: 'google', type: '', url: '' },
      ]}
      attempt={{ isFailed: false, isProcessing: false, message: '' }}
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
