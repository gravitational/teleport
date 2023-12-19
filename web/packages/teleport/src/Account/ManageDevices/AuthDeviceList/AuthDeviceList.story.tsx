import React from 'react';

import * as Icon from 'design/Icon';

import { ActionButton, Header } from 'teleport/Account/Header';

import { AuthDeviceList } from './AuthDeviceList';

export default {
  title: 'Teleport/Account/Manage Devices/Device List',
};

export function EmptyList() {
  return (
    <AuthDeviceList
      header={
        <Header
          title="Devices"
          description="Just some junk"
          icon={<Icon.ShieldCheck />}
          actions={
            <ActionButton>
              <Icon.Add />
              Add a new device
            </ActionButton>
          }
        />
      }
      devices={[]}
    />
  );
}

export function ListWithDevices() {
  return (
    <AuthDeviceList
      header={
        <Header
          title="Devices"
          description="These are very important devices, and I really need to provide a lengthy explanation for the reason why I'm listing them all here, just to make sure this text wraps to the new line, and ugh, I should have really just used some lorem ipsum."
          icon={<Icon.Key />}
          actions={
            <ActionButton>
              <Icon.Add />
              Add a new device
            </ActionButton>
          }
        />
      }
      devices={devices}
    />
  );
}

const devices = [
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
    name: 'solokey',
    registeredDate: new Date(1623722252000),
    lastUsedDate: new Date(1623981452000),
  },
  {
    id: '3',
    description: 'Hardware Key',
    name: 'backup yubikey',
    registeredDate: new Date(1618711052000),
    lastUsedDate: new Date(1626472652000),
  },
  {
    id: '4',
    description: 'Hardware Key',
    name: 'yubikey',
    registeredDate: new Date(1612493852000),
    lastUsedDate: new Date(1614481052000),
  },
];
