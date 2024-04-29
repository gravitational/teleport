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

import { Attempt } from 'shared/hooks/useAttemptNext';

export function isIamPermError(attempt: Attempt) {
  return (
    attempt.status === 'failed' &&
    attempt.statusText.includes('StatusCode: 403, RequestID:') &&
    attempt.statusText.includes('operation error')
  );
}

export function getAttemptsOneOfErrorMsg(attemptA: Attempt, attemptB: Attempt) {
  if (attemptA.status === 'failed') {
    return attemptA.statusText;
  }
  if (attemptB.status === 'failed') {
    return attemptB.statusText;
  }
  return '';
}
