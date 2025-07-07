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
