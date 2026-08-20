import { test } from '@gravitational/e2e/helpers/test';

test.use({ fixtures: ['ssh-node'] });

test('verify that a user can SSH into a node', async ({
  unifiedResourcesPage,
}) => {
  await unifiedResourcesPage.goto();

  const terminal = await unifiedResourcesPage.connect('docker-node', 'root');

  await terminal.waitForReady();
  await terminal.exec('ls /');
  await terminal.waitForText('bin');
});
