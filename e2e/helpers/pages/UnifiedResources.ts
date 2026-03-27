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

import { CLUSTER_NAME } from '../test';
import { TerminalPage } from './Terminal';

export class UnifiedResourcesPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto(`/web/cluster/${CLUSTER_NAME}/resources`);
  }

  /**
   * Click the "Connect" button on a resource, select a login from the
   * dropdown, and return the terminal popup wrapped in a TerminalPage.
   */
  async connect(serverName: string, login: string) {
    const row = this.page
      .locator('div')
      .filter({ hasText: serverName })
      .filter({ has: this.page.getByRole('button', { name: 'Connect' }) })
      .first();
    await row.getByRole('button', { name: 'Connect' }).click();

    const popupPromise = this.page.waitForEvent('popup');
    await this.page.getByRole('menuitem', { name: login }).click();
    const popup = await popupPromise;
    await popup.waitForLoadState('load');

    return new TerminalPage(popup);
  }
}
