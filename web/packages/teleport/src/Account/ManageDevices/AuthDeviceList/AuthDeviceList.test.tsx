import { render, screen } from 'design/utils/testing';
import { within } from '@testing-library/react';
import React from 'react';

import { MfaDevice } from 'teleport/services/mfa';

import { AuthDeviceList } from './AuthDeviceList';

const devices: MfaDevice[] = [
  {
    id: '1',
    description: 'Hardware Key',
    name: 'touch_id',
    registeredDate: new Date(1628799417000),
    lastUsedDate: new Date(1628799417000),
  },
  {
    id: '2',
    description: 'Hardware Key',
    name: 'yubikey',
    registeredDate: new Date(1623722252000),
    lastUsedDate: new Date(1623981452000),
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
  render(<AuthDeviceList header="Header" devices={devices} />);
  expect(screen.getByText('Header')).toBeInTheDocument();
  expect(getTableCellContents()).toEqual({
    header: ['Passkey Type', 'Nickname', 'Added', 'Last Used', 'Actions'],
    rows: [
      ['Hardware Key', 'touch_id', '2021-08-12', '2021-08-12', ''],
      ['Hardware Key', 'yubikey', '2021-06-15', '2021-06-18', ''],
    ],
  });
});

test('renders no devices', () => {
  render(<AuthDeviceList header="Header" devices={[]} />);
  expect(screen.getByText('Header')).toBeInTheDocument();
  expect(screen.queryAllByRole('row')).toEqual([]);
});
