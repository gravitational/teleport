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
      .locator('li')
      .filter({ has: this.page.getByText(serverName, { exact: true }) })
      .filter({ has: this.page.getByRole('button', { name: 'Connect' }) })
      .last();
    await row.getByRole('button', { name: 'Connect' }).click();

    const popupPromise = this.page.waitForEvent('popup');
    await this.page.getByRole('menuitem', { name: login }).click();
    const popup = await popupPromise;
    await popup.waitForLoadState('load');

    return new TerminalPage(popup);
  }

  async connectWithoutLogin(resourceName: string | RegExp): Promise<void> {
    const locator = this.page
      .locator('li')
      .filter({ hasText: resourceName })
      .filter({ has: this.page.getByRole('button', { name: 'Connect' }) })
      .first();

    await locator.getByRole('button', { name: 'Connect' }).click();
  }
}
