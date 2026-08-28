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

import { ChangedUserAuthn, RecoveryCodes } from './types';

// makeChangedUserAuthn makes the response from a successful user reset or invite.
// Only teleport cloud and users with valid emails as username will receive
// recovery codes.
export function makeChangedUserAuthn(json: any): ChangedUserAuthn {
  json = json || {};

  return {
    recovery: makeRecoveryCodes(json.recovery),
  };
}

export function makeRecoveryCodes(json: any): RecoveryCodes {
  json = json || {};

  return {
    codes: json.codes || [],
    createdDate: json.created ? new Date(json.created) : null,
  };
}
