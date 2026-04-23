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

import { test } from '@gravitational/e2e/helpers/test';

const missingSessionId = 'ae4038d7-e6c7-4dfb-92b5-fd03ac025e36';

test.describe('session recording not found', () => {
  test('ssh', async ({ playerPage }) => {
    await playerPage.goto(missingSessionId, 'ssh');
    await playerPage.expectError('Session recording not found');
  });

  test('k8s', async ({ playerPage }) => {
    await playerPage.goto(missingSessionId, 'k8s');
    await playerPage.expectError('Session recording not found');
  });

  test('desktop', async ({ playerPage }) => {
    await playerPage.goto(missingSessionId, 'desktop');
    await playerPage.expectError(
      'access denied to perform action "read" on "session"'
    );
  });

  test('database', async ({ playerPage }) => {
    await playerPage.goto(missingSessionId, 'database');
    await playerPage.expectError(
      'access denied to perform action "read" on "session"'
    );
  });
});
