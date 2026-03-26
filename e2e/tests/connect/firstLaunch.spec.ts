/**
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

import {
  expect,
  initializeDataDir,
  launchApp,
  test,
  withDefaultAppConfig,
} from '@gravitational/e2e/helpers/connect';

test('first launch shows usage data dialog', async () => {
  await using temp = await fs.mkdtempDisposable(
    path.join(os.tmpdir(), 'connect-e2e-first-launch-')
  );
  // Set usageReporting.enabled to undefined so it's not stored in the config, which causes the
  // usage data dialog to appear on launch.
  await initializeDataDir(
    temp.path,
    withDefaultAppConfig({ 'usageReporting.enabled': undefined })
  );

  await using app = await launchApp(temp.path);
  const { page } = app;

  const usageDataDialog = page.getByText('Anonymous usage data');
  await expect(usageDataDialog).toBeVisible();
  await page.getByRole('button', { name: 'Decline', exact: true }).click();

  // Assert the dialog is dismissed – without this, the "Connect a Cluster" check below would pass
  // even if clicking Decline failed, since that screen is already rendered under the modal.
  await expect(usageDataDialog).not.toBeVisible();

  // After dismissing the dialog, the app should show the default "Connect a Cluster" screen.
  await expect(page.getByText('Connect a Cluster')).toBeVisible();
});
