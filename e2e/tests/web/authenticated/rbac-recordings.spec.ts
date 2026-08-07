/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { expect, test } from '@gravitational/e2e/helpers/test';

test.describe('session list only', () => {
  test.use({
    user: {
      roles: [{ file: '@gravitational/e2e/roles/rbac-session-list.yaml' }],
      recordings: ['ssh-recording-1'],
    },
  });

  test('verify that playing a recorded session is denied without read access', async ({
    playerPage,
    recordingIds,
  }) => {
    await playerPage.goto(recordingIds['ssh-recording-1'], 'ssh');

    await expect(
      playerPage.getByText('Session recording not found')
    ).toBeVisible();
  });
});

test.describe('session list and read', () => {
  test.use({
    user: {
      roles: [{ file: '@gravitational/e2e/roles/rbac-session-read.yaml' }],
      recordings: ['ssh-recording-1'],
    },
  });

  test('verify that a user can replay a session with read access', async ({
    playerPage,
    recordingIds,
  }) => {
    await playerPage.goto(recordingIds['ssh-recording-1'], 'ssh');

    await expect(playerPage.terminal).toBeVisible();
  });
});
