/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { http, HttpResponse } from 'msw';

import {
  enableMswServer,
  render,
  screen,
  server,
  userEvent,
  waitFor,
} from 'design/utils/testing';

import { storageService } from 'teleport/services/storageService';

import { FormIdentifierFirst } from './FormIdentifierFirst';

enableMswServer();

test('no user remembered in localstorage renders username input form', () => {
  storageService.setRememberedSsoUsername('');

  render(
    <FormIdentifierFirst
      onLoginWithSso={jest.fn()}
      onUseLocalLogin={jest.fn()}
      isLocalAuthEnabled={true}
      title={'Sign in to Teleport'}
      ssoTitle={'Sign in to Teleport with SSO'}
    />
  );

  expect(
    screen.getByPlaceholderText('Username or email address')
  ).toBeVisible();
});

test('user remembered in localstorage shows welcome screen with connectors', async () => {
  storageService.setRememberedSsoUsername('joe@example.com');

  server.use(
    http.post('/v1/webapi/authconnectors', () =>
      HttpResponse.json(connectorsResp)
    )
  );

  render(
    <FormIdentifierFirst
      onLoginWithSso={jest.fn()}
      onUseLocalLogin={jest.fn()}
      isLocalAuthEnabled={true}
      title={'Sign in to Teleport'}
      ssoTitle={'Sign in to Teleport with SSO'}
    />
  );

  await expect(screen.getByText('Welcome, joe@example.com')).toBeVisible();
  await waitFor(() => {
    expect(screen.getByText(/Okta SSO/i)).toBeVisible();
  });
});

test('if there is only one connector returned after submitting username, the user is taken there immediately', async () => {
  storageService.setRememberedSsoUsername('');
  server.use(
    http.post('/v1/webapi/authconnectors', () =>
      HttpResponse.json(connectorsResp)
    )
  );

  const mockOnLoginWithSso = jest.fn();

  const user = userEvent.setup();

  render(
    <FormIdentifierFirst
      onLoginWithSso={mockOnLoginWithSso}
      onUseLocalLogin={jest.fn()}
      isLocalAuthEnabled={true}
      title={'Sign in to Teleport'}
      ssoTitle={'Sign in to Teleport with SSO'}
    />
  );

  const input = screen.getByPlaceholderText('Username or email address');
  await user.type(input, 'joe@example.com');
  await user.click(screen.getByRole('button', { name: 'Next' }));

  await waitFor(() => {
    expect(mockOnLoginWithSso).toHaveBeenCalled();
  });
});

const connectorsResp = {
  connectors: [
    {
      name: 'Okta',
      type: 'saml',
      displayName: 'Okta SSO',
      url: 'http://localhost/okta/login/web?redirect_url=http:%2F%2Flocalhost%2Fwebconnector_id=okta',
    },
  ],
};
