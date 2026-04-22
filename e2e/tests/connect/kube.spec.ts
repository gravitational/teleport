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

import { test, expect } from '@gravitational/e2e/helpers/connect';
import { TerminalPage } from '@gravitational/e2e/helpers/pages/Terminal';
import { UnifiedResourcesPage } from '@gravitational/e2e/helpers/pages/UnifiedResources';

test.use({ autoLogin: true, fixtures: ['kube'] });

const kubeResourceName = /teleport-e2e-kube-/i;
const kubePromptText = 'Try "kubectl version" to test the connection.';

async function openKubeTerminal(
  resources: UnifiedResourcesPage,
  terminal: TerminalPage
): Promise<void> {
  await resources.connectWithoutLogin(kubeResourceName);
  await terminal.waitForText(kubePromptText);
}

test('creates kubeconfig under user data and reuses for the same kube session', async ({
  app,
}) => {
  const { page, userDataDir } = app;
  const resources = new UnifiedResourcesPage(page);
  const terminal = new TerminalPage(page);

  await openKubeTerminal(resources, terminal);
  const kubeTab = page.locator(
    '[role="tab"][data-doc-kind="doc.gateway_kube"]'
  );
  await expect(kubeTab).toBeVisible();

  const pathBeforeTabClose = await terminal.execAndWait('echo $KUBECONFIG');
  expect(pathBeforeTabClose.startsWith(userDataDir)).toBe(true);

  await expect(kubeTab).toBeVisible();
  await kubeTab.locator('.close').click();
  await expect(kubeTab).toHaveCount(0);

  await openKubeTerminal(resources, terminal);
  await expect(kubeTab).toBeVisible();
  const pathAfterTabReopen = await terminal.execAndWait('echo $KUBECONFIG');

  expect(pathBeforeTabClose).toBe(pathAfterTabReopen);
});

test('closing connection removes kubeconfig file', async ({ app }) => {
  const { page } = app;
  const resources = new UnifiedResourcesPage(page);
  const terminal = new TerminalPage(page);

  await openKubeTerminal(resources, terminal);

  const kubeconfigPath = await terminal.execAndWait('echo $KUBECONFIG');

  expect(await fileExists(kubeconfigPath)).toBe(true);

  await page.getByTitle(/Open Connections/).click();
  const kubeConnection = page.locator('li').filter({ hasText: /KUBE/ }).first();
  await expect(kubeConnection).toBeVisible();

  const disconnectButton = kubeConnection.getByTitle(/Disconnect /).first();
  await expect(disconnectButton).toBeVisible();
  await disconnectButton.click();

  const removeButton = kubeConnection.getByTitle(/Remove /).first();
  await expect(removeButton).toBeVisible();
  await removeButton.click();

  await expect.poll(() => fileExists(kubeconfigPath)).toBe(false);
});

test('exec into a pod', async ({ app }) => {
  const { page } = app;
  const resources = new UnifiedResourcesPage(page);
  const terminal = new TerminalPage(page);
  const podName = `shell-demo-${crypto.randomUUID().split('-').at(0)}`;
  // Use `tsh kubectl` because `kubectl` is not available in CI.
  const kubectlCommand = `"$E2E_CONNECT_TSH_BIN" kubectl`;
  await openKubeTerminal(resources, terminal);

  const kubectlRunOutput = await terminal.execAndWait(
    `${kubectlCommand} run ${podName} --image=busybox:1.37.0 --command -- sh -c 'sleep 3600'`
  );
  expect(kubectlRunOutput).toContain(`pod/${podName} created`);

  await terminal.execAndWait(
    `${kubectlCommand} wait --for=condition=Ready pod/${podName} --timeout=20s`,
    { timeout: 20_000 }
  );
  await terminal.exec(
    `${kubectlCommand} exec --stdin --tty ${podName} -- /bin/sh`
  );
  await terminal.waitForText('/ #');
  // Check if the shell works.
  const whoamiOutput = await terminal.execAndWait('whoami');
  expect(whoamiOutput).toContain('root');
});

async function fileExists(filePath: string): Promise<boolean> {
  try {
    await fs.access(filePath);
    return true;
  } catch (e: unknown) {
    if (
      typeof e === 'object' &&
      e !== null &&
      'code' in e &&
      (e as { code?: string }).code === 'ENOENT'
    ) {
      return false;
    }
    throw e;
  }
}
