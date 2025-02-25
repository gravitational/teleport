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

import { render, screen } from 'design/utils/testing';

import { Manually } from './Manually';

test('loading state renders', async () => {
  const token = {
    id: 'token',
    expiryText: '',
    expiry: null,
    safeName: '',
    isStatic: false,
    method: 'kubernetes',
    roles: [],
    content: '',
  };
  render(
    <Manually
      isEnterprise={false}
      isAuthTypeLocal={true}
      version="v17"
      user="llama"
      token={token}
      attempt={{ status: 'processing' }}
      onClose={() => {}}
      createToken={() => null}
    />
  );

  await screen.findByTestId('indicator');
  expect(screen.queryByText(/step 1/i)).not.toBeInTheDocument();
});

test('success state renders', async () => {
  const token = {
    id: 'token',
    expiryText: '',
    expiry: null,
    safeName: '',
    isStatic: false,
    method: 'kubernetes',
    roles: [],
    content: '',
  };
  render(
    <Manually
      isEnterprise={false}
      isAuthTypeLocal={true}
      version="v17"
      user="llama"
      token={token}
      attempt={{ status: 'success' }}
      onClose={() => {}}
      createToken={() => null}
    />
  );

  expect(screen.getByText(/step 1/i)).toBeInTheDocument();
  expect(screen.getByText(/token will be valid for/i)).toBeInTheDocument();
  expect(screen.queryByText(/generate a join token/i)).not.toBeInTheDocument();
});

test('failed state renders', async () => {
  const token = {
    id: 'token',
    expiryText: '',
    expiry: null,
    safeName: '',
    isStatic: false,
    method: 'kubernetes',
    roles: [],
    content: '',
  };
  render(
    <Manually
      isEnterprise={false}
      isAuthTypeLocal={true}
      version="v17"
      user="llama"
      token={token}
      attempt={{ status: 'failed' }}
      onClose={() => {}}
      createToken={() => null}
    />
  );

  expect(screen.getByText(/step 1/i)).toBeInTheDocument();
  expect(screen.getByText(/generate a join token/i)).toBeInTheDocument();
  expect(
    screen.queryByText(/token will be valid for/i)
  ).not.toBeInTheDocument();
});
