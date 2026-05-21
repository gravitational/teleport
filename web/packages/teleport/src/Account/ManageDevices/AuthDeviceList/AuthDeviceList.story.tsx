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
import makeMfaDevice from 'teleport/services/mfa/makeMfaDevice';

import { AuthDeviceList } from './AuthDeviceList';

export default {
  title: 'Teleport/Account/Manage Devices/Auth Device List',
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
      devices={[]}
      attempt={{ status: 'success' }}
      passkeysEnabled
    />
  );
}

export function ListWithDevices({
  isPasswordlessEnabled,
}: {
  isPasswordlessEnabled: boolean;
}) {
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
      devices={devicesJson.map(d =>
        makeMfaDevice(d, { isPasswordlessEnabled })
      )}
      attempt={{ status: 'success' }}
      passkeysEnabled={isPasswordlessEnabled}
    />
  );
}

ListWithDevices.argTypes = {
  isPasswordlessEnabled: {
    control: 'boolean',
    defaultValue: true,
  },
};

ListWithDevices.args = { isPasswordlessEnabled: true };

const devicesJson: any[] = [
  {
    id: 'c62fcf79-a1ce-4774-9098-c13702bd7895',
    name: 'touch_id',
    addedAt: '2021-08-12T20:16:57.000Z',
    lastUsed: '2021-08-12T20:16:57.000Z',
    type: 'WebAuthn',
    residentKey: true,
  },
  {
    id: '44860895-b008-4b58-b283-978429d9980a',
    name: 'solokey',
    addedAt: '2021-06-15T01:57:32.000Z',
    lastUsed: '2021-06-18T01:57:32.000Z',
    type: 'WebAuthn',
    residentKey: true,
  },
  {
    id: 'ac7f91f7-a616-4e91-9d6d-652d35423377',
    name: 'authy',
    addedAt: '2021-04-18T01:57:32.000Z',
    lastUsed: '2021-07-16T21:57:32.000Z',
    type: 'TOTP',
    residentKey: false,
  },
  {
    id: '50d90d05-67ee-4fdd-be54-2f5182fea9a5',
    name: 'yubikey',
    addedAt: '2021-02-05T02:57:32.000Z',
    lastUsed: '2021-02-28T02:57:32.000Z',
    type: 'WebAuthn',
    residentKey: false,
  },
  {
    id: '5cc6a573-e9d2-4d8d-a34e-566c91b51233',
    name: 'okta',
    addedAt: '2021-02-05T02:57:32.000Z',
    lastUsed: '2021-02-28T02:57:32.000Z',
    type: 'SSO',
    residentKey: false,
  },
];
