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

import type { Page } from '@playwright/test';

export class RolesPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto('/web/roles');
  }

  async createRole(name: string) {
    await this.page.getByRole('button', { name: 'Create New Role' }).click();
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
    await this.page.getByRole('button', { name: 'Create Role' }).click();
  }

  async editRole(name: string) {
    await this.page
      .getByRole('row', { name: `${name} Options` })
      .getByRole('button')
      .click();
    await this.page.getByRole('menuitem', { name: 'Edit' }).click();
  }

  async deleteRole(name: string) {
    await this.page.getByRole('row', { name }).getByRole('button').click();
    await this.page.getByRole('menuitem', { name: 'Delete' }).click();
    await this.page.getByRole('button', { name: 'Yes, Remove Role' }).click();
  }
}
