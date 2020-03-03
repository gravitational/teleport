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
import Login from './Login';
import { render, fireEvent, wait } from 'design/utils/testing';
import auth from '../../services/auth/auth';
import { Auth2faTypeEnum, AuthProviderTypeEnum } from '../../services/enums';
import cfg from 'teleport/config';
import history from '../../services/history';

beforeEach(() => {
  jest.spyOn(history, 'push').mockImplementation();
  jest.spyOn(history, 'getRedirectParam').mockImplementation(() => '/');
});

afterEach(() => {
  jest.clearAllMocks();
});

test('basic rendering', () => {
  const { container, getByText } = render(<Login />);

  // test rendering of logo and title
  expect(container.querySelector('img')).toBeInTheDocument();
  expect(getByText(/sign into teleport/i)).toBeInTheDocument();
});

test('login with username/password', async () => {
  jest.spyOn(auth, 'login').mockResolvedValue();

  const { getByPlaceholderText, getByText } = render(<Login />);

  // test validation errors
  fireEvent.click(getByText(/login/i));
  expect(auth.login).not.toHaveBeenCalled();

  // fill form
  const username = getByPlaceholderText(/user name/i);
  const password = getByPlaceholderText(/password/i);
  fireEvent.change(username, { target: { value: 'username' } });
  fireEvent.change(password, { target: { value: '123' } });

  // test login pathways
  await wait(() => fireEvent.click(getByText(/login/i)));
  expect(auth.login).toHaveBeenCalledWith('username', '123', '');
  expect(history.push).toHaveBeenCalledWith('http://localhost/web', true);
});

test('login with U2F', async () => {
  jest.spyOn(auth, 'loginWithU2f').mockResolvedValue();
  jest
    .spyOn(cfg, 'getAuth2faType')
    .mockImplementation(() => Auth2faTypeEnum.UTF);

  const { getByPlaceholderText, getByText } = render(<Login />);

  // test validation errors
  fireEvent.click(getByText(/login/i));
  expect(auth.loginWithU2f).not.toHaveBeenCalled();

  // fill form
  const username = getByPlaceholderText(/user name/i);
  const password = getByPlaceholderText(/password/i);
  fireEvent.change(username, { target: { value: 'username' } });
  fireEvent.change(password, { target: { value: '123' } });

  // test login pathways
  await wait(() => fireEvent.click(getByText(/login/i)));
  expect(auth.loginWithU2f).toHaveBeenCalledWith('username', '123');
  expect(history.push).toHaveBeenCalledWith('http://localhost/web', true);
});

test('login with SSO', () => {
  jest
    .spyOn(cfg, 'getAuth2faType')
    .mockImplementation(() => Auth2faTypeEnum.OTP);
  jest.spyOn(cfg, 'getAuthProviders').mockImplementation(() => [
    {
      type: AuthProviderTypeEnum.GITHUB,
      name: AuthProviderTypeEnum.GITHUB,
      url:
        '/github/login/web?redirect_url=:redirect?connector_id=:providerName',
    },
  ]);

  const { getByText } = render(<Login />);

  // test login pathways
  fireEvent.click(getByText(AuthProviderTypeEnum.GITHUB));
  expect(history.push).toHaveBeenCalledWith(
    'http://localhost/github/login/web?redirect_url=http:%2F%2Flocalhost%2Fwebconnector_id=github',
    true
  );
});
