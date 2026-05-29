/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { expect, test } from '@gravitational/e2e/helpers/test';

// Text in the terminal that the test will attempt to copy to the clipboard
const textToCopy = 'text-to-copy';
// Text that is pre-populated in the clipboard before the test's copy attempt
const preCopiedText = 'pre-copied-text';

test.describe('web terminal clipboard mode', () => {
  test.use({
    fixtures: ['ssh-node'],
    permissions: ['clipboard-read', 'clipboard-write'],
  });

  test.describe('with web_terminal_clipboard_mode: no-copy', () => {
    test.use({
      user: {
        roles: [
          'access',
          'editor',
          { file: '@gravitational/e2e/roles/web-terminal-no-copy.yaml' },
        ],
      },
    });

    test('cannot copy terminal text to the clipboard', async ({
      unifiedResourcesPage,
    }) => {
      await unifiedResourcesPage.goto();

      const terminal = await unifiedResourcesPage.connect('docker-node', 'root');
      await terminal.waitForReady();

      await terminal.exec(`echo ${textToCopy}`);
      await terminal.waitForText(textToCopy);

      // Pre-populate the clipboard with the preCopiedText
      await terminal.writeClipboard(preCopiedText);

      await terminal.copyAllText();

      const clipboard = await terminal.readClipboard();
      expect(clipboard).toBe(preCopiedText);
      expect(clipboard).not.toContain(textToCopy);
    });
  });

  test.describe('with unrestricted clipboard', () => {
    test.use({
      user: {
        roles: ['access', 'editor'],
      },
    });

    test('can copy terminal text to the clipboard', async ({
      unifiedResourcesPage,
    }) => {
      await unifiedResourcesPage.goto();

      const terminal = await unifiedResourcesPage.connect('docker-node', 'root');
      await terminal.waitForReady();

      await terminal.exec(`echo ${textToCopy}`);
      await terminal.waitForText(textToCopy);

      // Reset the clipboard so we know the copy is what populated it.
      await terminal.writeClipboard('');

      await terminal.copyAllText();

      await expect.poll(() => terminal.readClipboard()).toContain(textToCopy);
    });
  });
});
