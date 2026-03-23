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
  login,
  test,
  withDefaultAppConfig,
} from '@gravitational/e2e/helpers/connect';

// These tests manage the app lifecycle manually (multiple launches/closes), so they do not use the
// `app` fixture.
test.describe('state restoration from disk', () => {
  // Run all tests in this block sequentially in a single worker so that beforeAll runs exactly
  // once, not once per test (which fullyParallel would otherwise cause).
  test.describe.configure({ mode: 'serial' });

  // A snapshot of a data dir with a logged-in session, created once in beforeAll. Tests that need
  // logged-in state copy this snapshot instead of calling login again, keeping total login calls to
  // a minimum (important to avoid rate-limiting when running full Connect suite).
  let loggedInSnapshotPath: string;

  test.beforeAll(async () => {
    loggedInSnapshotPath = await fs.mkdtemp(
      path.join(os.tmpdir(), 'connect-e2e-state-snapshot-')
    );
    await initializeDataDir(loggedInSnapshotPath, withDefaultAppConfig({}));

    // Login once to populate the data dir with session state.
    {
      await using app = await launchApp(loggedInSnapshotPath);
      await login(app.page);
    }
  });

  test.afterAll(async () => {
    await fs.rm(loggedInSnapshotPath, { recursive: true, force: true });
  });

  /**
   * Creates a fresh disposable temp dir pre-populated with the logged-in snapshot.
   */
  async function loggedInDataDir() {
    const temp = await fs.mkdtempDisposable(
      path.join(os.tmpdir(), 'connect-e2e-state-')
    );
    await fs.cp(loggedInSnapshotPath, temp.path, { recursive: true });
    return temp;
  }

  test('relaunch restores tabs', async () => {
    await using temp = await loggedInDataDir();

    // Open a terminal tab so there are 2 tabs (cluster + terminal).
    {
      await using app = await launchApp(temp.path);
      const { page } = app;

      await page.getByTitle('Additional Actions').click();
      await page.getByText('Open new terminal').click();
      await expect(
        page.getByRole('textbox', { name: 'Terminal input' })
      ).toBeVisible();
    }

    // Relaunch – the app should offer to restore the terminal tab.
    {
      await using app = await launchApp(temp.path);
      const { page } = app;
      await expect(page.getByText('Reopen previous session')).toBeVisible();
      await page.getByRole('button', { name: 'Reopen' }).click();
      await expect(page.getByText('Reopen previous session')).not.toBeVisible();

      // Both tabs should be restored.
      await expect(
        page.locator('[role="tab"][data-doc-kind="doc.cluster"]')
      ).toBeVisible();
      await expect(
        page.locator('[role="tab"][data-doc-kind="doc.terminal_shell"]')
      ).toBeVisible();
    }
  });

  test('missing app_state.json does not crash the app', async () => {
    await using temp = await loggedInDataDir();

    // Remove app_state.json (keep tsh dir) – the app should not crash.
    const appStatePath = path.join(temp.path, 'userData', 'app_state.json');
    await fs.rm(appStatePath);

    {
      await using app = await launchApp(temp.path);

      // Without app_state.json, the app has no saved rootClusterUri, so no workspace is activated
      // and the cluster connect panel prompts the user to pick one.
      await expect(
        app.page.getByText('Log in to a cluster to use Teleport Connect.')
      ).toBeVisible();
    }
  });

  test('missing tsh home directory does not crash the app', async () => {
    await using temp = await loggedInDataDir();

    // Remove the tsh home directory (keep app_state.json) – the app should not crash.
    const tshHomePath = path.join(temp.path, 'home', '.tsh');
    await fs.rm(tshHomePath, { recursive: true });

    {
      await using app = await launchApp(temp.path);
      const { page } = app;

      // With no tsh dir, no cluster can be connected.
      await expect(page.getByText('Connect a Cluster')).toBeVisible();
      // app_state.json still references the old cluster, so setActiveWorkspace should show an error.
      await expect(
        page.getByText('Could not set cluster as active')
      ).toBeVisible();
    }
  });

  test('logout clears previous tabs', async () => {
    await using temp = await loggedInDataDir();
    await using app = await launchApp(temp.path);
    const { page } = app;

    // Open a terminal tab.
    await page.getByTitle('Additional Actions').click();
    await page.getByText('Open new terminal').click();
    await expect(
      page.getByRole('textbox', { name: 'Terminal input' })
    ).toBeVisible();

    // Logout.
    await page.getByTitle(/Open Profiles/).click();
    await page.getByTitle(/Log out/).click();
    await expect(
      page.getByText('Are you sure you want to log out?')
    ).toBeVisible();
    await page.getByRole('button', { name: 'Log Out', exact: true }).click();
    await expect(page.getByText('Connect a Cluster')).toBeVisible();

    // Login to the same cluster again.
    await login(page);

    // No restore dialog should appear – logout clears previous tabs.
    // Only the default cluster tab should be open.
    const clusterTab = page.locator(
      '[role="tab"][data-doc-kind="doc.cluster"]'
    );
    await expect(clusterTab).toHaveCount(1);
    await expect(
      page.locator('[role="tab"][data-doc-kind="doc.terminal_shell"]')
    ).toHaveCount(0);
    await expect(page.getByText('Reopen previous session')).not.toBeVisible();
  });

  test('identical workspace shape does not trigger restore dialog', async () => {
    await using temp = await loggedInDataDir();

    // Replace the default cluster tab with a new one (same shape).
    {
      await using app = await launchApp(temp.path);
      const { page } = app;

      const clusterTab = page.locator(
        '[role="tab"][data-doc-kind="doc.cluster"]'
      );
      await expect(clusterTab).toBeVisible();

      // Close the existing cluster tab.
      await clusterTab.locator('.close').click();
      // Open a new cluster tab via the "+" button.
      await page.getByTitle(/New Tab/).click();
      await expect(clusterTab).toBeVisible();
    }

    // Relaunch – no restore dialog since the workspace has the same shape.
    {
      await using app = await launchApp(temp.path);
      const { page } = app;

      const clusterTab = page.locator(
        '[role="tab"][data-doc-kind="doc.cluster"]'
      );
      await expect(clusterTab).toHaveCount(1);
      await expect(page.getByText('Reopen previous session')).not.toBeVisible();
    }
  });

  test('window remembers size and position after restart', async () => {
    await using temp = await fs.mkdtempDisposable(
      path.join(os.tmpdir(), 'connect-e2e-state-')
    );
    await initializeDataDir(temp.path, withDefaultAppConfig({}));

    // Launch the app and resize the window.
    let targetBounds: {
      x: number;
      y: number;
      width: number;
      height: number;
    };
    {
      await using app = await launchApp(temp.path);
      const { page } = app;
      await expect(page.getByText('Connect a Cluster')).toBeVisible();

      // Pick bounds relative to the primary display's work area so the test works on any screen
      // size and on multi-monitor setups where the primary display may be offset from (0, 0).
      // WindowsManager.getWindowState restores saved bounds only when they fit entirely within
      // a display.
      const requestedBounds = await app.electronApp.evaluate(({ screen }) => {
        const wa = screen.getPrimaryDisplay().workArea;
        return {
          x: wa.x + Math.floor(wa.width * 0.1),
          y: wa.y + Math.floor(wa.height * 0.1),
          width: Math.floor(wa.width * 0.6),
          height: Math.floor(wa.height * 0.6),
        };
      });

      // Capture the initial bounds before resizing so we can verify setBounds() had an effect.
      const initialBounds = await app.electronApp.evaluate(
        ({ BrowserWindow }) => {
          const win = BrowserWindow.getAllWindows()[0];
          return win.getNormalBounds();
        }
      );

      await app.electronApp.evaluate(({ BrowserWindow }, bounds) => {
        const win = BrowserWindow.getAllWindows()[0];
        win.setBounds(bounds);
      }, requestedBounds);

      // Read back the accepted bounds after the window manager has applied them. The WM may adjust
      // the requested rectangle (e.g. on Wayland or tiling layouts), and the app persists
      // getNormalBounds(), not the requested values.
      targetBounds = await app.electronApp.evaluate(({ BrowserWindow }) => {
        const win = BrowserWindow.getAllWindows()[0];
        return win.getNormalBounds();
      });

      // Verify the window actually moved. On environments where setBounds() is a no-op (e.g.
      // Wayland, tiling WMs), the relaunch assertion would pass trivially without this guard.
      expect(targetBounds).not.toEqual(initialBounds);
    }

    // Relaunch – the window should restore to the same size & position.
    {
      await using app = await launchApp(temp.path);

      // Wait for the app to fully initialize before reading bounds and closing, otherwise the
      // app may close before the renderer acks the initial cluster store message, causing a
      // "Failed to receive message acknowledgement from the renderer" error dialog.
      await expect(app.page.getByText('Connect a Cluster')).toBeVisible();

      const bounds = await app.electronApp.evaluate(({ BrowserWindow }) => {
        const win = BrowserWindow.getAllWindows()[0];
        return win.getNormalBounds();
      });

      expect(bounds).toEqual(targetBounds);
    }
  });
});
