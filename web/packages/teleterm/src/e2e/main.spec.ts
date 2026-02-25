import { _electron as electron, expect, test } from '@playwright/test';

import { wait } from 'shared/utils/wait';

test('log in', async () => {
  const { app, page } = await appReady();
  expect(page.getByText('Connect a Cluster')).toBeVisible();
  await page.getByRole('button', { name: 'Connect', exact: true }).click();
  await page
    .getByPlaceholder('teleport.example.com')
    .fill('teleport-18-ent.teleport.town');
  await page.getByRole('button', { name: 'Next' }).click();
  await page.getByRole('button', { name: 'Sign in with GitHub' }).click();
  await expect(page.getByPlaceholder('Search or jump to…')).toBeVisible({
    timeout: 10_000,
  });
  await app.close();
});

test('connect to database', async () => {
  const { app, page } = await appReady();
  await page.getByPlaceholder('Search or jump to…').fill('aurora');
  await page.getByText('Set up a db connection to aurora').click();
  await page.getByText('teleport-user').click();
  await expect(page.getByText('Database Connection')).toBeVisible();
  await page.getByRole('button', { name: 'Run' }).click();
  // test the connection to the db.
  await app.close();
});

test('connect to kube', async () => {
  const { app, page } = await appReady();
  await page.getByPlaceholder('Search or jump to…').fill('cookie');
  await page.getByText('Log in to Kubernetes cluster cookie').click();
  await expect(
    page.getByText('Started a local proxy for Kubernetes cluster "cookie".')
  ).toBeVisible();
  await app.close();
});

test('open terminal and verifies cluster env vars', async () => {
  const { app, page } = await appReady();
  await page.getByTitle(/Additional Actions/).click();
  await page.getByText(/Open new terminal/).click();
  await page.focus('.xterm-viewport');

  await page.keyboard.insertText('echo $TELEPORT_CLUSTER');
  await page.keyboard.press('Enter');
  await expect(page.getByTitle('teleport-18-ent.teleport.town')).toBeVisible();

  await page.keyboard.insertText('echo $TELEPORT_PROXY');
  await page.keyboard.press('Enter');
  await expect(page.getByTitle('teleport-18-ent.teleport.town')).toBeVisible();

  await app.close();
});

test('log out', async () => {
  const { app, page } = await appReady();
  await page.getByTitle(/Open Profiles/).click();
  await page.getByTitle(/Log out/).click();
  await expect(
    page.getByText('Are you sure you want to log out?')
  ).toBeVisible();
  await page.getByRole('button', { name: 'Log Out', exact: true }).click();
  await expect(page.getByText('Connect a Cluster')).toBeVisible();
  await app.close();
});

async function appReady() {
  const app = await electron.launch({ args: ['.'] });
  const page = await app.firstWindow();
  await page.waitForLoadState('domcontentloaded');
  // Needed because the dialog is opened from useEffect.
  // TODO find a better way.
  await wait(100);
  if (await page.getByText('Reopen previous session').isVisible()) {
    await page
      .getByRole('button', { name: 'Start New Session', exact: true })
      .click();
  }
  return { app, page };
}
