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

import { State as AttemptState } from 'shared/hooks/useAttemptNext';

export type LockResource = {
  kind: LockResourceKind;
  // targetValue is the value used
  // in making a lock request.
  targetValue: string;
  // friendlyName is the name that the user
  // will see on the screen instead of the
  // targetValue if defined (eg: instead of showing user
  // node id, we show node hostname which is easier to read)
  friendlyName?: string;
};

export type ToggleSelectResourceFn = (resource: LockResource) => void;

export type CommonListProps = {
  pageSize: number;
  attempt: AttemptState['attempt'];
  setAttempt: AttemptState['setAttempt'];
  selectedResourceKind: LockResourceKind;
  selectedResources: LockResourceMap;
  toggleSelectResource: ToggleSelectResourceFn;
};

// ResourceKind describes which resource kinds can be locked.
export type LockResourceKind =
  | 'user'
  | 'role'
  | 'login'
  | 'node'
  | 'server_id'
  | 'mfa_device'
  | 'windows_desktop'
  | 'access_request'
  | 'device'; // trusted devices

type TargetValue = string;
type FriendlyName = string;

// ResourceMap will be used to keep track of all the resource
// name the user selects to lock.
export type LockResourceMap = {
  [K in LockResourceKind]: Record<TargetValue, TargetValue | FriendlyName>;
};

export function getEmptyResourceMap(): LockResourceMap {
  return {
    node: {},
    windows_desktop: {},
    role: {},
    user: {},
    mfa_device: {},
    login: {},
    access_request: {},
    device: {},
    server_id: {},
  };
}

type ListKind =
  // simple refers to lists where paginating and filtering are handled
  // on the client side. Resources like users, roles, mfa devices,
  // access requests still retrieve everything up front.
  | 'simple'
  // hybrid refers to lists with partial server side support for
  // paging (supply start key and limit) and the unsupported
  // filtering/searching is done on the client side.
  | 'hybrid'
  // server-side refers to lists with pure server side paginating and
  // filtering support (eg: nodes, databases, desktops, etc.)
  | 'server-side'
  // logins is special in that we can't fetch logins from the back.
  // this kind of list requires manual input from users.
  | 'logins';

export type LockResourceOption = {
  value: LockResourceKind;
  label: string;
  listKind: ListKind;
};

export const baseResourceKindOpts: LockResourceOption[] = [
  {
    value: 'user',
    label: 'Users',
    listKind: 'simple',
  },
  {
    value: 'role',
    label: 'Roles',
    listKind: 'server-side',
  },
  {
    value: 'mfa_device',
    label: 'MFA Devices',
    listKind: 'simple',
  },
  {
    value: 'login',
    label: 'Logins',
    listKind: 'logins',
  },
  {
    value: 'node',
    label: 'Servers',
    listKind: 'server-side',
  },
  {
    value: 'windows_desktop',
    label: 'Desktops',
    listKind: 'server-side',
  },
];
