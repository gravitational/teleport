/**
 * Copyright 2020-2022 Gravitational, Inc.
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
import { render, fireEvent, waitFor, screen } from 'design/utils/testing';
import auth from 'teleport/services/auth/auth';
import history from 'teleport/services/history';
import cfg from 'teleport/config';
import Login from './Login';

beforeEach(() => {
  jest.spyOn(history, 'push').mockImplementation();
  jest.spyOn(history, 'getRedirectParam').mockImplementation(() => '/');
  jest.resetAllMocks();
});

test('basic rendering', () => {
  const { container, getByText } = render(<Login />);

  // test rendering of logo and title
  expect(container.querySelector('img')).toBeInTheDocument();
  expect(getByText(/sign into teleport/i)).toBeInTheDocument();
});

test('login with username and password', async () => {
  jest.spyOn(auth, 'login').mockResolvedValue(null);

  const { getByPlaceholderText, getByText } = render(<Login />);

  // test validation errors
  fireEvent.click(getByText(/login/i));
  expect(auth.login).not.toHaveBeenCalled();

  // fill form
  const username = getByPlaceholderText(/username/i);
  const password = getByPlaceholderText(/password/i);
  fireEvent.change(username, { target: { value: 'username' } });
  fireEvent.change(password, { target: { value: '123' } });

  // test login pathways
  await waitFor(() => fireEvent.click(getByText(/login/i)));
  expect(auth.login).toHaveBeenCalledWith('username', '123', '');
  expect(history.push).toHaveBeenCalledWith('http://localhost/web', true);
});

test('login with password and otp', async () => {
  jest.spyOn(auth, 'login').mockResolvedValue(null);
  jest.spyOn(cfg, 'getAuth2faType').mockImplementation(() => 'otp');

  render(<Login />);

  // fill form
  const username = screen.getByPlaceholderText(/username/i);
  const password = screen.getByPlaceholderText(/password/i);
  fireEvent.change(username, { target: { value: 'username' } });
  fireEvent.change(password, { target: { value: '123' } });

  // test token requirement
  fireEvent.click(screen.getByText(/login/i));
  screen.getByText(/token is require/i);
  expect(auth.login).not.toHaveBeenCalled();

  // test login pathway
  const token = screen.getByPlaceholderText('123 456');
  fireEvent.change(token, { target: { value: '0' } });

  await waitFor(() => fireEvent.click(screen.getByText(/login/i)));
  expect(auth.login).toHaveBeenCalledWith('username', '123', '0');
  expect(history.push).toHaveBeenCalledWith('http://localhost/web', true);
});

test('login with password and webauthn', async () => {
  jest.spyOn(auth, 'loginWithWebauthn').mockResolvedValue(null);
  jest.spyOn(cfg, 'getAuth2faType').mockImplementation(() => 'webauthn');

  const { getByPlaceholderText, getByText } = render(<Login />);

  // test validation errors
  fireEvent.click(getByText(/login/i));
  expect(auth.loginWithWebauthn).not.toHaveBeenCalled();

  // fill form
  const username = getByPlaceholderText(/username/i);
  const password = getByPlaceholderText(/password/i);
  fireEvent.change(username, { target: { value: 'username' } });
  fireEvent.change(password, { target: { value: '123' } });

  // test login pathways
  await waitFor(() => fireEvent.click(getByText(/login/i)));
  expect(auth.loginWithWebauthn).toHaveBeenCalledWith('username', '123');
  expect(history.push).toHaveBeenCalledWith('http://localhost/web', true);
});

test('login with SSO', () => {
  jest.spyOn(cfg, 'getAuth2faType').mockImplementation(() => 'otp');
  jest.spyOn(cfg, 'getAuthProviders').mockImplementation(() => [
    {
      displayName: 'With Github',
      type: 'github',
      name: 'github',
      url: '/github/login/web?redirect_url=:redirect?connector_id=:providerName',
    },
  ]);

  const { getByText } = render(<Login />);

  // test login pathways
  fireEvent.click(getByText('With Github'));
  expect(history.push).toHaveBeenCalledWith(
    'http://localhost/github/login/web?redirect_url=http:%2F%2Flocalhost%2Fwebconnector_id=github',
    true
  );
});

test('login with 2fa set to "optional", select option: none', async () => {
  jest.spyOn(auth, 'login').mockResolvedValue(null);
  jest.spyOn(cfg, 'getAuth2faType').mockImplementation(() => 'optional');

  render(<Login />);

  // select none from mfa dropdown
  const selectEl = screen.getByTestId('mfa-select').querySelector('input');
  fireEvent.focus(selectEl);
  fireEvent.keyDown(selectEl, { key: 'ArrowDown', keyCode: 40 });
  fireEvent.click(screen.getByText(/none/i));

  // fill form
  const username = screen.getByPlaceholderText(/username/i);
  const password = screen.getByPlaceholderText(/password/i);
  fireEvent.change(username, { target: { value: 'username' } });
  fireEvent.change(password, { target: { value: '123' } });

  await waitFor(() => fireEvent.click(screen.getByText(/login/i)));
  expect(auth.login).toHaveBeenCalledWith('username', '123', '');
});

test('2fa set to "on" with preferredMfa renders hardware key as first in dropdown', async () => {
  jest.spyOn(cfg, 'getAuth2faType').mockImplementation(() => 'on');
  jest.spyOn(cfg, 'getPreferredMfaType').mockImplementation(() => 'webauthn');

  const { container } = render(<Login />);
  expect(container).toMatchSnapshot();
});
