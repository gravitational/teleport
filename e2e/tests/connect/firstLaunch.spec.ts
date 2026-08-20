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
