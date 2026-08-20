import fs from 'node:fs/promises';

import {
  expect,
  test,
  withDefaultAppConfig,
  withRawAppConfig,
} from '@gravitational/e2e/helpers/connect';

test.describe('config applies valid values', () => {
  test.use({
    autoLogin: true,
    appConfig: withDefaultAppConfig({
      'terminal.fontFamily': 'Courier New',
    }),
  });

  test('terminal font change', async ({ app }) => {
    const { page } = app;
    await page.getByTitle('Additional Actions').click();
    await page.getByRole('button', { name: 'Open new terminal' }).click();

    const fontFamily = await page
      .getByTestId('terminal-container')
      .last()
      .evaluate(el => el.style.fontFamily);
    expect(fontFamily).toBe('"Courier New"');
  });
});

test.describe('config duplicate shortcuts', () => {
  const duplicateShortcut =
    process.platform === 'darwin' ? 'Command+1' : 'Ctrl+1';

  test.use({
    appConfig: withDefaultAppConfig({
      'keymap.tab1': duplicateShortcut,
      'keymap.tab2': duplicateShortcut,
    }),
  });

  test('duplicate keyboard shortcuts show a conflict notification', async ({
    app,
  }) => {
    const { page } = app;
    const duplicateShortcut =
      process.platform === 'darwin' ? 'Command+1' : 'Ctrl+1';
    await expect(
      page.getByText('Shortcuts conflicts', { exact: true })
    ).toBeVisible();
    await expect(
      page.getByText(
        `${duplicateShortcut} is used for actions: tab1, tab2. Only one of them will work.`,
        { exact: true }
      )
    ).toBeVisible();
  });
});

test.describe('config invalid value', () => {
  test.use({
    appConfig: withDefaultAppConfig({
      'keymap.tab1': 'ABC',
    }),
  });

  test('invalid config value shows validation notification', async ({
    app,
  }) => {
    const { page } = app;
    await expect(
      page.getByText('Encountered errors in config file', {
        exact: true,
      })
    ).toBeVisible();
    await expect(
      page.getByText('keymap.tab1: "ABC" cannot be used as a key code')
    ).toBeVisible();
  });
});

test.describe('config malformed JSON', () => {
  const malformedConfig = 'not-a-json';

  test.use({
    appConfig: withRawAppConfig(malformedConfig),
  });

  test('syntax error shows an error notification, the config file contents is not overridden', async ({
    app,
  }) => {
    const { page, electronApp, appConfigPath } = app;
    await expect(
      page.getByText('Failed to load config file', { exact: true })
    ).toBeVisible();

    // Close the app and verify the config on disk has not been overridden.
    await electronApp.close();

    const contentAfterClose = await fs.readFile(appConfigPath, 'utf8');
    expect(contentAfterClose).toBe(malformedConfig);
  });
});
