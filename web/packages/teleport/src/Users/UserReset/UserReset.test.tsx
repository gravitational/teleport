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

import { render, screen } from 'design/utils/testing';
import { Attempt } from 'shared/hooks/useAttemptNext';

import cfg from 'teleport/config';
import { User } from 'teleport/services/user';

import { UserReset } from './UserReset';

test('local user reset dialog shows option to get a reset url', () => {
  render(
    <UserReset
      user={{ name: 'alice', isLocal: true } as User}
      response={null}
      onReset={() => {}}
      onClose={() => {}}
      attempt={{ status: '' } as Attempt}
    />
  );

  expect(
    screen.getByRole('button', { name: /Generate Reset URL/ })
  ).toBeInTheDocument();
  expect(
    screen.queryByRole('button', { name: /Remove MFA Devices/ })
  ).not.toBeInTheDocument();
});

test('sso user reset dialog shows option to reset MFA devices', () => {
  render(
    <UserReset
      user={{ name: 'bob', isLocal: false } as User}
      response={null}
      onReset={() => {}}
      onClose={() => {}}
      attempt={{ status: '' } as Attempt}
    />
  );

  expect(
    screen.getByRole('button', { name: /Reset MFA Devices/ })
  ).toBeInTheDocument();
  expect(
    screen.queryByRole('button', { name: /Generate Reset URL/ })
  ).not.toBeInTheDocument();
});

test('sso user success dialog shows how many MFA devices were removed', () => {
  render(
    <UserReset
      user={{ name: 'bob', isLocal: false } as User}
      response={{ userKind: 'sso', mfaDevicesRemoved: 2 }}
      onReset={() => {}}
      onClose={() => {}}
      attempt={{ status: 'success' } as Attempt}
    />
  );

  expect(screen.getByText(/Removed 2 MFA devices/)).toBeInTheDocument();
});

test('local user success dialog shows the reset url', () => {
  const token = {
    value: 'abc123',
    username: 'alice',
    expires: new Date(Date.now() + 100000),
  };

  render(
    <UserReset
      user={{ name: 'alice', isLocal: true } as User}
      response={{ token, userKind: 'local', mfaDevicesRemoved: 0 }}
      onReset={() => {}}
      onClose={() => {}}
      attempt={{ status: 'success' } as Attempt}
    />
  );

  expect(
    screen.getByText(/has been reset\. Share this URL/)
  ).toBeInTheDocument();
  expect(
    screen.getByText(cfg.getUserResetTokenRoute('abc123', false))
  ).toBeInTheDocument();
});
