/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

export function buildPredicateExpression(
  requestedStatuses: string[],
  requestedPredicateExpression: string = ''
): string {
  let statusPredicate = '';

  if (requestedStatuses && requestedStatuses.length) {
    // deduplicate and ignore empty strings
    const seenMap = {};
    statusPredicate = requestedStatuses
      .filter(status => {
        if (status && !seenMap[status]) {
          seenMap[status] = true;
          return true;
        }
      })
      .map(status => `health.status == "${status}"`)
      .join(' || ');
  }

  let newPredicateExpression = requestedPredicateExpression;

  if (!newPredicateExpression) {
    newPredicateExpression = statusPredicate;
  } else if (statusPredicate != '') {
    newPredicateExpression = `${newPredicateExpression} && (${statusPredicate})`;
  }

  return newPredicateExpression;
}
