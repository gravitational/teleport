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

export class RolesPage {
  readonly createNewRoleButton: Locator;
  readonly createRoleButton: Locator;
  readonly saveChangesButton: Locator;
  readonly yamlEditorTab: Locator;
  readonly editor: Locator;
  readonly infoGuideButton: Locator;
  readonly firstOptionsMenuButton: Locator;

  constructor(private page: Page) {
    this.createNewRoleButton = page.getByTestId('create_new_role_button');

    this.createRoleButton = page.getByRole('button', { name: 'Create Role' });

    this.saveChangesButton = page.getByRole('button', {
      name: 'Save Changes',
    });

    this.yamlEditorTab = page.getByRole('tab', {
      name: 'Switch to YAML editor',
    });

    this.editor = page.locator('.ace_editor');

    this.infoGuideButton = page.getByTestId('info-guide-btn-open');

    this.firstOptionsMenuButton = page
      .locator('table')
      .getByRole('button')
      .first();
  }

  async goto() {
    await this.page.goto('/web/roles');
  }

  async createRole(name: string) {
    await this.createNewRoleButton.click();

    await this.page
      .getByRole('textbox', { name: 'Role Name(required)' })
      .fill(name);

    await this.page.getByRole('button', { name: 'Next: Resources' }).click();

    await this.page
      .getByRole('button', { name: 'Add Teleport Resource Access' })
      .click();

    await this.page
      .getByRole('menuitem', { name: 'SSH Server Access' })
      .click();

    await this.page.getByRole('button', { name: 'Next: Admin Rules' }).click();
    await this.page.getByRole('button', { name: 'Next: Options' }).click();

    await this.createRoleButton.click();
  }

  async editRole(name: string) {
    await this.openOptionsMenu(name);

    await this.page.getByRole('menuitem', { name: 'Edit' }).click();
  }

  async deleteRole(name: string) {
    await this.openOptionsMenu(name);

    await this.page.getByRole('menuitem', { name: 'Delete' }).click();

    await this.page.getByRole('button', { name: 'Yes, Remove Role' }).click();
  }

  async switchToYamlEditor() {
    await this.yamlEditorTab.click();

    await this.editor.waitFor({ state: 'visible' });
  }

  async replaceYaml(search: string, replacement: string) {
    await this.page.evaluate(
      ({ search, replacement }) => {
        const editor = (window as any).ace.edit(
          document.querySelector('.ace_editor')
        );
        const content = editor.session.getValue();

        editor.session.setValue(content.replace(search, replacement));
      },
      { search, replacement }
    );
  }

  async setYaml(content: string) {
    await this.editor.click();

    await this.page.keyboard.press('ControlOrMeta+a');
    await this.page.keyboard.type(content);
  }

  private async openOptionsMenu(roleName: string) {
    await this.page
      .getByRole('row', { name: roleName })
      .getByRole('button')
      .click();
  }
}
