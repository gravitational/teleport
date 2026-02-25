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

import { expect, test } from '@playwright/test';

test('verify that a user can SSH into a node', async ({ page }) => {
  await page.goto('/web/cluster/teleport-e2e/resources');
  await page.getByRole('button', { name: 'Connect' }).click();

  const terminalPromise = page.waitForEvent('popup');
  await page.getByRole('menuitem', { name: 'root' }).click();
  const terminal = await terminalPromise;

  const terminalInput = terminal.getByRole('textbox', {
    name: 'Terminal input',
  });
  await expect(terminalInput).toBeVisible();

  const xtermRows = terminal.locator('.xterm-rows');

  await expect(xtermRows).not.toHaveText('');

  await terminalInput.pressSequentially('ls /\n');
  await expect(xtermRows).toContainText('usr');
});
