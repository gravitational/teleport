/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { within } from '@testing-library/react';

import { render, screen, userEvent } from 'design/utils/testing';

import { MfaDevice } from 'teleport/services/mfa';

import { AuthDeviceList } from './AuthDeviceList';

const devices: MfaDevice[] = [
  {
    id: '1',
    description: 'Passkey',
    name: 'touch_id',
    registeredDate: new Date(1628799417000),
    lastUsedDate: new Date(1628799417000),
    type: 'webauthn',
    usage: 'passwordless',
  },
  {
    id: '2',
    description: 'Hardware Key',
    name: 'yubikey',
    registeredDate: new Date(1623722252000),
    lastUsedDate: new Date(1623981452000),
    type: 'webauthn',
    usage: 'mfa',
  },
];

const ssoDevice: MfaDevice[] = [
  {
    id: '1',
    description: 'SSO Provider',
    name: 'okta',
    registeredDate: new Date(1628799417000),
    lastUsedDate: new Date(1628799417000),
    type: 'sso',
    usage: 'mfa',
  },
];

function getTableCellContents() {
  const [header, ...rows] = screen.getAllByRole('row');
  return {
    header: within(header)
      .getAllByRole('columnheader')
      .map(cell => cell.textContent.trim()),
    rows: rows.map(row =>
      within(row)
        .getAllByRole('cell')
        .map(cell => cell.textContent.trim())
    ),
  };
}

// The device column stacks the nickname over the device type. Reading them as separate lines keeps the
// assertions legible and catches a type that should have been suppressed as a duplicate.
function deviceRow(rowIndex: number) {
  return within(screen.getAllByRole('row')[rowIndex + 1]);
}

function expectDeviceLines(rowIndex: number, nickname: string, type: string) {
  expect(deviceRow(rowIndex).getByText(nickname)).toBeVisible();
  expect(deviceRow(rowIndex).getByText(type)).toBeVisible();
}

// The type line is suppressed when it repeats the nickname, so the text appears exactly once.
function expectNicknameOnly(rowIndex: number, nickname: string) {
  expect(deviceRow(rowIndex).getAllByText(nickname)).toHaveLength(1);
}

test('renders devices', () => {
  render(
    <AuthDeviceList
      header="Header"
      devices={devices}
      attempt={{ status: 'success' }}
      passkeysEnabled
    />
  );
  expect(screen.getByText('Header')).toBeInTheDocument();
  const { header, rows } = getTableCellContents();
  expect(header).toEqual(['Device', 'Added', 'Last Used', 'Actions']);
  expect(rows.map(cells => cells.slice(1))).toEqual([
    ['2021-08-12', '2021-08-12', ''],
    ['2021-06-15', '2021-06-18', ''],
  ]);
  expectDeviceLines(0, 'touch_id', 'Passkey');
  expectDeviceLines(1, 'yubikey', 'Hardware Key');

  const buttons = screen.queryAllByTitle('Delete');
  expect(buttons).toHaveLength(2);
  // all buttons should be enabled
  buttons.forEach(button => {
    expect(button).toBeEnabled();
  });
  // No additional info icons expected
  expect(
    screen.queryAllByRole('graphics-symbol', { name: 'information' })
  ).toHaveLength(0);
});

test('renders devices with passkeys disabled', async () => {
  const user = userEvent.setup();

  render(
    <AuthDeviceList
      header="Header"
      devices={devices}
      attempt={{ status: 'success' }}
      passkeysEnabled={false}
    />
  );

  const infoIcons = screen.getAllByRole('graphics-symbol', {
    name: 'Information',
  });
  expect(infoIcons).toHaveLength(1);
  await user.hover(infoIcons[0]);
  expect(
    screen.getByText(
      'This device can be a passkey, but passwordless authentication is disabled'
    )
  ).toBeVisible();
});

test('delete button is disabled for sso devices', () => {
  render(
    <AuthDeviceList
      header="Header"
      devices={ssoDevice}
      attempt={{ status: 'success' }}
      passkeysEnabled
    />
  );
  expect(screen.getByText('Header')).toBeInTheDocument();
  const { header, rows } = getTableCellContents();
  expect(header).toEqual(['Device', 'Added', 'Last Used', 'Actions']);
  expect(rows.map(cells => cells.slice(1))).toEqual([
    ['2021-08-12', '2021-08-12', ''],
  ]);
  expectDeviceLines(0, 'okta', 'SSO Provider');

  const button = screen.getByTitle('SSO device cannot be deleted');
  expect(button).toBeInTheDocument();
  expect(button).toBeDisabled();
});

test('omits the device type when the nickname already matches it', () => {
  // adce0002 resolves to "Chrome on Mac", which is also what the passkey wizard suggests as a nickname.
  const chromeAaguid = 'adce0002-35bc-c60a-648b-0b25f1f05503';
  render(
    <AuthDeviceList
      header="Header"
      devices={[
        { ...devices[0], name: 'Chrome on Mac', aaguid: chromeAaguid },
        {
          ...devices[1],
          name: 'work laptop',
          aaguid: chromeAaguid,
          registeredDate: new Date(1623722252000),
        },
        {
          ...devices[1],
          id: '3',
          name: 'Hardware Key',
          registeredDate: new Date(1600000000000),
        },
      ]}
      attempt={{ status: 'success' }}
      passkeysEnabled
    />
  );
  expectNicknameOnly(0, 'Chrome on Mac');
  expectDeviceLines(1, 'work laptop', 'Chrome on Mac');
  expectNicknameOnly(2, 'Hardware Key');
});

test('sorting the device column groups by type, then nickname', async () => {
  const user = userEvent.setup();
  const chromeAaguid = 'adce0002-35bc-c60a-648b-0b25f1f05503'; // "Chrome on Mac"
  render(
    <AuthDeviceList
      header="Header"
      devices={[
        { ...devices[1], id: '1', name: 'zeta', aaguid: chromeAaguid },
        { ...devices[1], id: '2', name: 'alpha', description: 'Hardware Key' },
        { ...devices[1], id: '3', name: 'beta', aaguid: chromeAaguid },
      ]}
      attempt={{ status: 'success' }}
      passkeysEnabled
    />
  );

  await user.click(screen.getByText('Device'));

  // Chrome on Mac before Hardware Key, and alphabetical within the group.
  expectDeviceLines(0, 'beta', 'Chrome on Mac');
  expectDeviceLines(1, 'zeta', 'Chrome on Mac');
  expectDeviceLines(2, 'alpha', 'Hardware Key');
});

test('renders no devices', () => {
  render(
    <AuthDeviceList
      header="Header"
      devices={[]}
      attempt={{ status: 'success' }}
      passkeysEnabled
    />
  );
  expect(screen.getByText('Header')).toBeInTheDocument();
  expect(screen.queryAllByRole('row')).toEqual([]);
});
