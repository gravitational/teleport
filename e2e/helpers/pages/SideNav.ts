import type { Locator, Page } from '@playwright/test';

export class SideNavPage {
  constructor(private page: Page) {}

  async openSection(sectionName: string): Promise<Locator> {
    const button = this.page.getByRole('button', { name: sectionName });

    await button.hover();
    await button.click();

    const panel = this.page.locator(`[id="panel-${sectionName}"]`);

    await panel.waitFor({ state: 'visible' });

    return panel;
  }

  async navigateToLink(sectionName: string, linkName: string) {
    const panel = await this.openSection(sectionName);

    await panel.getByRole('link', { name: linkName }).click();

    await this.page.waitForLoadState('networkidle');
  }
}
