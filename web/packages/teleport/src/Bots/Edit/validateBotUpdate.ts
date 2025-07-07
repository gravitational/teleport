/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { EditBotRequest, FlatBot } from 'teleport/services/bot/types';

import { formatDuration } from '../formatDuration';

/**
 * Given a previous and next bot object, plus the update request, validateBotUpdate checks
 * if the fields included in the request are present in the resultant bot object. Max session
 * duration is sensitive to user input, while roles and traits are not. As such, the
 * check may return a false positive in some situations. For example, if the user changed
 * the previous value of max_session_ttl from "12h" to "43200s" (which are equivalent),
 * the check will see that as a change that is not present in the updated bot object.
 *
 * @param prev the bot before the update
 * @param request the update/edit request data
 * @param next the bot after the update
 * @returns boolean indicating whether the update was valid
 */
export function validateBotUpdate(
  prev: FlatBot | null | undefined,
  request: EditBotRequest,
  next: FlatBot
) {
  const inconsistentFields: string[] = [];

  if (request.roles) {
    if (!arrayLooseEqual(request.roles, next.roles)) {
      inconsistentFields.push('roles');
    }
  }

  if (request.traits) {
    if (request.traits.length !== next.traits.length) {
      inconsistentFields.push('traits');
    } else {
      for (const trait of request.traits) {
        const match = next.traits.find(t => t.name === trait.name);
        if (!match) {
          inconsistentFields.push('traits');
          break;
        }

        if (!arrayLooseEqual(match.values, trait.values)) {
          inconsistentFields.push('traits');
          break;
        }
      }
    }
  }

  if (request.max_session_ttl) {
    const matchReq =
      request.max_session_ttl === formatDuration(next.max_session_ttl);
    const matchPrev =
      prev?.max_session_ttl?.seconds === next.max_session_ttl?.seconds;
    if (!matchReq && matchPrev) {
      inconsistentFields.push('max_session_ttl');
    }
  }

  return inconsistentFields;
}

function arrayLooseEqual(a: string[], b: string[]) {
  if (a.length !== b.length) return false;
  return a.every(r => b.includes(r));
}
