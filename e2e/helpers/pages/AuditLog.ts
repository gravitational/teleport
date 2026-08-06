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

import { CLUSTER_NAME, expect } from '../test';

const EVENT_TIMEOUT = 30_000;

export class AuditLogPage {
  private readonly searchInput;

  constructor(private page: Page) {
    this.searchInput = page.getByPlaceholder('Search...');
  }

  async goto() {
    await this.page.goto(`/web/cluster/${CLUSTER_NAME}/audit`);
    await expect(
      this.page.getByRole('heading', { name: 'Audit Log' })
    ).toBeVisible();
  }

  async search(term: string) {
    await this.searchInput.fill(term);
    await this.searchInput.press('Enter');

    await expect(this.page).toHaveURL(new RegExp(`search=${term}`));
  }

  /**
   * Waits for an event matching both the event type and the search term to appear in the log,
   * reloading between attempts. Events are written asynchronously and the page caches its query
   * results, so an event that lands after the first fetch only shows up on a reload.
   */
  async waitForEvent(eventType: string, searchTerm: string): Promise<Locator> {
    const row = this.page
      .getByRole('row')
      .filter({ hasText: eventType })
      .filter({ hasText: searchTerm })
      .first();

    await expect(async () => {
      await this.page.reload();
      await expect(row).toBeVisible({ timeout: 5_000 });
    }).toPass({ timeout: EVENT_TIMEOUT });

    return row;
  }

  /**
   * Opens an event's details dialog and returns the raw event JSON it renders.
   */
  async eventJSON(row: Locator): Promise<string> {
    await row.getByRole('button', { name: 'Details' }).click();

    const dialog = this.page.getByRole('dialog');
    await expect(dialog).toBeVisible();

    const lines = await dialog.locator('.ace_line').allInnerTexts();

    await dialog.getByRole('button', { name: 'Close' }).click();
    await expect(dialog).toBeHidden();

    return lines.join('\n');
  }

  /**
   * Returns the value an event's JSON gives for a field, or undefined when the field is not there.
   */
  async eventField(row: Locator, field: string): Promise<string | undefined> {
    const json = await this.eventJSON(row);
    const match = new RegExp(`"${field}":\\s*([^,\\n]+)`).exec(json);

    return match?.[1].trim();
  }

  /**
   * Asserts the log holds no event of this type for the search term. Only sound once a later event
   * from the same session is already visible, otherwise it passes simply by being early.
   */
  async expectNoEvent(eventType: string, searchTerm: string) {
    await this.page.reload();

    await expect(
      this.page
        .getByRole('row')
        .filter({ hasText: eventType })
        .filter({ hasText: searchTerm })
    ).toHaveCount(0);
  }
}
