import fs from 'node:fs/promises';

import { test, expect } from '@gravitational/e2e/helpers/connect';
import { TerminalPage } from '@gravitational/e2e/helpers/pages/Terminal';
import { UnifiedResourcesPage } from '@gravitational/e2e/helpers/pages/UnifiedResources';

test.use({
  autoLogin: true,
  fixtures: ['kube'],
  user: {
    roles: ['access'],
    traits: { kubernetes_groups: ['system:masters'] },
  },
});

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
  // Use `tsh kubectl` because `kubectl` is not available in CI.
  const kubectlCommand = `"$E2E_CONNECT_TSH_BIN" kubectl`;
  await openKubeTerminal(resources, terminal);

  // Reuse kind's kube-proxy pod to test exec without pulling an additional image.
  const versionOutput = await terminal.execAndWait(
    `${kubectlCommand} exec --stdin --tty --namespace kube-system daemonset/kube-proxy -- /usr/local/bin/kube-proxy --version`
  );
  expect(versionOutput).toContain('Kubernetes v');
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
