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

import type { Locator, Page } from '@playwright/test';

export class TrustedClustersPage {
  readonly connectButton: Locator;
  readonly editButton: Locator;
  readonly saveChangesButton: Locator;
  readonly editor: Locator;

  constructor(private page: Page) {
    this.connectButton = page.getByRole('button', {
      name: 'Connect to Root Cluster',
    });

    this.editButton = page.getByRole('button', {
      name: 'Edit Trusted Cluster',
    });

    this.saveChangesButton = page.getByRole('button', {
      name: 'Save changes',
    });

    this.editor = page.locator('.ace_editor');
  }

  async goto() {
    await this.page.goto('/web/trust');
  }

  async appendToYamlAndSave() {
    await this.editButton.click();

    await this.editor.waitFor({ state: 'visible' });

    await this.page.evaluate(() => {
      const editor = (window as any).ace.edit(
        document.querySelector('.ace_editor')
      );

      editor.session.setValue(editor.session.getValue() + '\n');
    });

    await this.saveChangesButton.click();
  }
}
