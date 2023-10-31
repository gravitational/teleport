/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
    loggedInUser.userType === tsh.UserType.USER_TYPE_UNSPECIFIED ||
    !loggedInUser.acl
  ) {
    return { status: 'unknown' };
  }

  if (loggedInUser.userType === tsh.UserType.USER_TYPE_SSO) {
    return { status: 'no-access', reason: 'sso-user' };
  }

  if (!loggedInUser?.acl?.tokens.create) {
    return { status: 'no-access', reason: 'insufficient-permissions' };
  }

  return { status: 'ok' };
}
