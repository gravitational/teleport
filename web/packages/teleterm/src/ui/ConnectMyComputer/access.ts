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

import { RuntimeSettings } from 'teleterm/mainProcess/types';
import * as tsh from 'teleterm/services/tshd/types';

export type ConnectMyComputerAccess =
  | {
      /**
       * "unknown" means that cluster details weren't fetched yet and we cannot perform definitive
       * checks.
       */
      status: 'unknown';
    }
  | { status: 'ok' }
  | ConnectMyComputerAccessNoAccess;

export type ConnectMyComputerAccessNoAccess =
  | { status: 'no-access'; reason: 'unsupported-platform' }
  | { status: 'no-access'; reason: 'insufficient-permissions' }
  | { status: 'no-access'; reason: 'sso-user' };

// TODO(gzdunek): we should have a single place where all permissions are defined.
// This will make it easier to understand what the user can and cannot do without having to jump around the code base.
// https://github.com/gravitational/teleport/pull/28346#discussion_r1246653846
export function getConnectMyComputerAccess(
  loggedInUser: tsh.LoggedInUser,
  runtimeSettings: RuntimeSettings
): ConnectMyComputerAccess {
  const isUnix =
    runtimeSettings.platform === 'darwin' ||
    runtimeSettings.platform === 'linux';

  if (!isUnix) {
    return { status: 'no-access', reason: 'unsupported-platform' };
  }

  if (
    !loggedInUser ||
    loggedInUser.userType === tsh.LoggedInUser_UserType.UNSPECIFIED ||
    !loggedInUser.acl
  ) {
    return { status: 'unknown' };
  }

  if (loggedInUser.userType === tsh.LoggedInUser_UserType.SSO) {
    return { status: 'no-access', reason: 'sso-user' };
  }

  if (!loggedInUser?.acl?.tokens.create) {
    return { status: 'no-access', reason: 'insufficient-permissions' };
  }

  return { status: 'ok' };
}
