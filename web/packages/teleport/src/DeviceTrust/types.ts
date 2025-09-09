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

import { PagerPosition } from 'design/DataTable/types';

export type TrustedDevice = {
  id: string;
  assetTag: string;
  osType: TrustedDeviceOSType;
  enrollStatus: 'enrolled' | 'not enrolled';
  owner: string;
  createTime?: Date;
  /**
   * source indicates how the device got into Teleport's inventory.
   * Devices registered directly via Teleport through tctl have no assigned source.
   */
  source?: DeviceSource;
};

/**
 * DeviceSource indicates how the device got into Teleport's inventory.
 * Devices registered directly via Teleport through tctl have no assigned source.
 */
export type DeviceSource = {
  /**
   * name is an arbitrary name that can be set when using either the Jamf integration [1] or when
   * registering devices through the Teleport API.
   *
   * When using the Jamf or Intune integrations with default settings, it's set to "jamf" and
   * "intune" respectively.
   *
   * [1]: https://goteleport.com/docs/identity-governance/device-trust/jamf-integration/#optional-sync-multiple-sources
   */
  name: string;
  origin: DeviceOrigin;
};

export type TrustedDeviceOSType = 'Windows' | 'Linux' | 'macOS';

export type TrustedDeviceResponse = {
  items: TrustedDevice[];
  startKey: string;
};

export type DeviceListProps = {
  items: TrustedDeviceResponse['items'];
  pageSize?: number;
  pagerPosition?: PagerPosition;
  fetchStatus?: 'loading' | 'disabled' | '';
  fetchData?: () => void;
};

export enum DeviceOrigin {
  Unspecified = 0,
  /**
   * Api is only ever set if a customer wrote a 3rd-party solution for syncing devices and uses
   * Teleport's API directly. The customer would be steered into using this origin, as setting the
   * origin and the name is required by the API (see e/lib/devicetrust/storage.ValidateDeviceForCreate).
   */
  Api,
  Jamf,
  Intune,
}

export function deviceSource(source: DeviceSource | undefined): string {
  if (!source) {
    return '';
  }

  // Display the name only if it's not equal to the default name used by the given origin.
  if (source.name == originToDefaultName[source.origin]) {
    return originToPrettyName[source.origin];
  }

  return source.name;
}

/**
 * originToDefaultName maps an origin to the default name each MDM integration uses as its source
 * name.
 */
const originToDefaultName: Partial<Record<DeviceOrigin, string>> = {
  [DeviceOrigin.Jamf]: 'jamf',
  [DeviceOrigin.Intune]: 'intune',
};

const originToPrettyName: Partial<Record<DeviceOrigin, string>> = {
  [DeviceOrigin.Jamf]: 'Jamf',
  [DeviceOrigin.Intune]: 'Intune',
};
