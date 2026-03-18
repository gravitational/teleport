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

test('verify that a user can SSH into a node', async ({
  unifiedResourcesPage,
}) => {
  await unifiedResourcesPage.goto();

  const terminal = await unifiedResourcesPage.connect('docker-node', 'root');

  await terminal.waitForReady();
  await terminal.exec('ls /');
  await terminal.waitForText('bin');
});
