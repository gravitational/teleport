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

import * as Icon from 'design/Icon';

import { ActionButtonSecondary, Header } from 'teleport/Account/Header';
import { MfaDevice } from 'teleport/services/mfa';

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
            <ActionButtonSecondary>
              <Icon.Add />
              Add a new device
            </ActionButtonSecondary>
          }
        />
      }
      deviceTypeColumnName="Passkey Type"
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
            <ActionButtonSecondary>
              <Icon.Add />
              Add a new device
            </ActionButtonSecondary>
          }
        />
      }
      deviceTypeColumnName="Passkey Type"
      devices={devices}
    />
  );
}

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
    name: 'solokey',
    registeredDate: new Date(1623722252000),
    lastUsedDate: new Date(1623981452000),
    type: 'webauthn',
    usage: 'passwordless',
  },
  {
    id: '3',
    description: 'Hardware Key',
    name: 'backup yubikey',
    registeredDate: new Date(1618711052000),
    lastUsedDate: new Date(1626472652000),
    type: 'webauthn',
    usage: 'passwordless',
  },
  {
    id: '4',
    description: 'Hardware Key',
    name: 'yubikey',
    registeredDate: new Date(1612493852000),
    lastUsedDate: new Date(1614481052000),
    type: 'webauthn',
    usage: 'passwordless',
  },
  {
    id: '5',
    description: 'sso provider',
    name: 'okta',
    registeredDate: new Date(1612493852000),
    lastUsedDate: new Date(1614481052000),
    type: 'sso',
    usage: 'mfa',
  },
];
