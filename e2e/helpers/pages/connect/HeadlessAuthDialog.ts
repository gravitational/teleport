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

import { expect } from '@gravitational/e2e/helpers/test';
import type { Page } from '@playwright/test';

export class HeadlessAuthDialogPage {
  private readonly dialog;

  constructor(page: Page) {
    this.dialog = page
      .getByRole('dialog')
      .filter({ hasText: 'Headless command on' });
  }

  async waitForVisible() {
    await expect(this.dialog).toBeVisible();
  }

  async waitForClose() {
    await expect(this.dialog).toBeHidden();
  }

  async waitForRequestId(requestId: string) {
    await expect(this.dialog).toContainText(requestId);
  }

  async approve() {
    await this.waitForVisible();
    await this.dialog.getByRole('button', { name: 'Approve' }).click();
  }

  async reject() {
    await this.waitForVisible();
    await this.dialog.getByRole('button', { name: 'Reject' }).click();
  }

  async close() {
    await this.waitForVisible();
    await this.dialog.getByRole('button', { name: /close/i }).click();
  }
}
