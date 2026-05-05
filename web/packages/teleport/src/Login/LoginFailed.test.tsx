/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { MemoryRouter } from 'react-router';

import { render, screen } from 'design/utils/testing';

import cfg from 'teleport/config';

import { LoginFailed } from './LoginFailed';

test('callback error path shows callback error', () => {
  render(
    <MemoryRouter initialEntries={[cfg.routes.loginErrorCallback]}>
      <LoginFailed />
    </MemoryRouter>
  );
  expect(
    screen.getByText('Unable to process SSO callback.')
  ).toBeInTheDocument();
});

test('missing role shows missing role error', () => {
  render(
    <MemoryRouter initialEntries={[cfg.routes.loginErrorCallbackMissingRole]}>
      <LoginFailed />
    </MemoryRouter>
  );
  expect(
    screen.getByText(
      'Unable to process SSO callback. The connector has a mapping to a role that does not exist. Please contact your SSO administrator.'
    )
  ).toBeInTheDocument();
});

test('unauthorized error path shows unauthorized error', () => {
  render(
    <MemoryRouter initialEntries={[cfg.routes.loginErrorUnauthorized]}>
      <LoginFailed />
    </MemoryRouter>
  );
  expect(
    screen.getByText(
      'You are not authorized, please contact your SSO administrator.'
    )
  ).toBeInTheDocument();
});

test('groups overage error path shows groups overage error', () => {
  render(
    <MemoryRouter initialEntries={[cfg.routes.loginErrorEntraIDGroupsOverage]}>
      <LoginFailed />
    </MemoryRouter>
  );
  expect(
    screen.getByText(
      'Your account is a member of more than 150 Entra ID groups. Please contact your SSO administrator to configure Graph API access on the Teleport SAML connector.'
    )
  ).toBeInTheDocument();
});

test('unhandled error path shows generic error', () => {
  render(
    <MemoryRouter initialEntries={['/web/msg/error/login/some_unhandled_path']}>
      <LoginFailed />
    </MemoryRouter>
  );
  expect(
    screen.getByText(
      "Unable to log in, please check Teleport's log for details."
    )
  ).toBeInTheDocument();
});
