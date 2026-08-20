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
  await expect(terminalInput).toBeVisible();

  await terminalInput.pressSequentially(
    `echo $TELEPORT_PROXY $TELEPORT_CLUSTER $TELEPORT_AUTH_SERVER $TELEPORT_TOOLS_VERSION\n`
  );
  await expect(terminal).toContainText(
    `${proxyHost} ${clusterName} ${proxyHost} off`
  );

  // Verify that TELEPORT_HOME points to the tsh home directory managed by Connect.
  await terminalInput.pressSequentially('echo $TELEPORT_HOME\n');
  await expect(terminal).toContainText('/home/.tsh');

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
    new RegExp(`${realTmpDir} · ${clusterName}$`)
  );

  // Verify that `exit` (code 0) closes the tab.
  await terminalInput.pressSequentially('exit\n');
  await expect(terminalTab).toHaveCount(0);

  // Open a new terminal to test `exit 1`.
  await page.getByTitle('Additional Actions').click();
  await page.getByText('Open new terminal').click();
  await expect(terminalInput).toBeVisible();

  // Verify that `exit 1` does not close the tab but the shell exits
  // (the tab title loses the cwd, leaving just the cluster name or shell name + cluster name).
  await terminalInput.pressSequentially('exit 1\n');
  await expect(terminalTab).toHaveAccessibleName(new RegExp(`${clusterName}$`));
});
