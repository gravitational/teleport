/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
import { mockWebAuthn } from '@gravitational/e2e/helpers/webauthn';

// TODO(@rudream): re-enable this once the UI bug is fixed.
test.skip('verify that a user can create and delete an auth connector', async ({
  page,
}) => {
  await mockWebAuthn(page);
  await page.goto('/');
  await page.getByRole('button', { name: 'Zero Trust Access' }).click();
  await page.getByRole('link', { name: 'Auth Connectors' }).click();
  await page.getByRole('button', { name: 'New GitHub Connector' }).click();

  await page.waitForSelector('.ace_editor', { state: 'visible' });
  await page.evaluate(() => {
    const editor = (window as any).ace.edit(
      document.querySelector('.ace_editor')
    );

    const lines = editor.session.getDocument().getAllLines();

    lines[3] = '  name: testconnector';

    editor.session.setValue(lines.join('\n'));
  });

  await page.getByRole('button', { name: 'Save Changes' }).click();

  await expect(page.getByText('testconnector')).toBeVisible();

  const connectorTile = page.getByTestId('testconnector-tile');

  await connectorTile.getByRole('button').click();

  await page.getByRole('menuitem', { name: 'Delete' }).click();
  await page.getByRole('button', { name: 'Delete Connector' }).click();

  await page.waitForTimeout(500);

  await expect(page.getByText('testconnector')).not.toBeVisible();
});
