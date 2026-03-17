/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';

import { test, expect } from '@gravitational/e2e/helpers/connect';
import { startUrl } from '@gravitational/e2e/helpers/env';

test.use({ autoLogin: true });

test('shell session', async ({ app }) => {
  const { page } = app;

  const proxyHost = new URL(startUrl).host;
  // Read the cluster name from the default DocumentCluster tab.
  const clusterTab = page.locator('[role="tab"][data-doc-kind="doc.cluster"]');
  const clusterName = await clusterTab.getAttribute('title');
  if (!clusterName) {
    throw new Error('Cluster tab is missing a title');
  }

  await page.getByTitle('Additional Actions').click();
  await page.getByText('Open new terminal').click();

  const terminal = page.locator('.xterm');
  const terminalInput = page.getByRole('textbox', { name: 'Terminal input' });
  await expect(terminalInput).toBeVisible({ timeout: 10_000 });

  await terminalInput.pressSequentially(
    `echo $TELEPORT_PROXY $TELEPORT_CLUSTER $TELEPORT_AUTH_SERVER\n`
  );
  await expect(terminal).toContainText(
    `${proxyHost} ${clusterName} ${proxyHost}`,
    {
      timeout: 10_000,
    }
  );

  // Verify that changing directory updates the tab title.
  await using tmpDir = await fs.mkdtempDisposable(
    path.join(os.tmpdir(), 'connect-e2e-shell-')
  );
  // Resolve symlinks (e.g., on macOS /tmp -> /private/tmp) to match what the shell reports.
  const realTmpDir = await fs.realpath(tmpDir.path);
  await terminalInput.pressSequentially(`cd ${realTmpDir}\n`);
  const terminalTab = page.locator(
    '[role="tab"][data-doc-kind="doc.terminal_shell"]'
  );
  await expect(terminalTab).toHaveAccessibleName(
    new RegExp(`${realTmpDir} · ${clusterName}$`),
    { timeout: 10_000 }
  );

  // Verify that `exit` (code 0) closes the tab.
  await terminalInput.pressSequentially('exit\n');
  await expect(terminalTab).toHaveCount(0, { timeout: 10_000 });

  // Open a new terminal to test `exit 1`.
  await page.getByTitle('Additional Actions').click();
  await page.getByText('Open new terminal').click();
  await expect(terminalInput).toBeVisible({ timeout: 10_000 });

  // Verify that `exit 1` does not close the tab but the shell exits
  // (the tab title loses the cwd, leaving just the cluster name or shell name + cluster name).
  await terminalInput.pressSequentially('exit 1\n');
  await expect(terminalTab).toHaveAccessibleName(
    new RegExp(`${clusterName}$`),
    { timeout: 10_000 }
  );
});
