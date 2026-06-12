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

import { render, screen } from 'design/utils/testing';

import { MfaDevice } from 'teleport/services/mfa';

import { AuthDeviceList } from './AuthDeviceList';

const devices: MfaDevice[] = [
  {
    id: '1',
    description: 'Hardware Key',
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
    usage: 'passwordless',
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
      .map(cell => cell.textContent),
    rows: rows.map(row =>
      within(row)
        .getAllByRole('cell')
        .map(cell => cell.textContent)
    ),
  };
}

test('renders devices', () => {
  render(
    <AuthDeviceList
      header="Header"
      deviceTypeColumnName="Passkey Type"
      devices={devices}
    />
  );
  expect(screen.getByText('Header')).toBeInTheDocument();
  expect(getTableCellContents()).toEqual({
    header: ['Passkey Type', 'Nickname', 'Added', 'Last Used', 'Actions'],
    rows: [
      ['Hardware Key', 'touch_id', '2021-08-12', '2021-08-12', ''],
      ['Hardware Key', 'yubikey', '2021-06-15', '2021-06-18', ''],
    ],
  });

  const buttons = screen.queryAllByTitle('Delete');
  expect(buttons).toHaveLength(2);
  // all buttons should be enabled
  buttons.forEach(button => {
    expect(button).toBeEnabled();
  });
});

test('delete button is disabled for sso devices', () => {
  render(
    <AuthDeviceList
      header="Header"
      deviceTypeColumnName="Passkey Type"
      devices={ssoDevice}
    />
  );
  expect(screen.getByText('Header')).toBeInTheDocument();
  expect(getTableCellContents()).toEqual({
    header: ['Passkey Type', 'Nickname', 'Added', 'Last Used', 'Actions'],
    rows: [['SSO Provider', 'okta', '2021-08-12', '2021-08-12', '']],
  });

  const button = screen.getByTitle('SSO device cannot be deleted');
  expect(button).toBeInTheDocument();
  expect(button).toBeDisabled();
});

test('renders no devices', () => {
  render(
    <AuthDeviceList
      deviceTypeColumnName="Passkey Type"
      header="Header"
      devices={[]}
    />
  );
  expect(screen.getByText('Header')).toBeInTheDocument();
  expect(screen.queryAllByRole('row')).toEqual([]);
});
