import { expect, Locator, Page } from '@playwright/test';

/**
 * A page object that exposes elements common to typical authenticated pages.
 */
export class AuthenticatedPage {
  readonly userMenu: Locator;
  readonly accountSettingsMenuItem: Locator;
  readonly logoutMenuItem: Locator;

  constructor(public readonly page: Page) {
    this.userMenu = page.getByRole('button', { name: 'User Menu' });
    this.userMenu = page.getByRole('button', { name: 'User Menu' });
    this.accountSettingsMenuItem = page.getByRole('link', {
      name: 'Account Settings',
    });
    this.logoutMenuItem = page.getByRole('menuitem', { name: 'Logout' });
  }

  /**
   * Navigates to the main page. Using goto is optional for using this page
   * object, as most authenticated pages are compatible; this method just uses
   * a sane default.
   */
  async goto() {
    await this.page.goto('/');
  }

  async openAccountSettings() {
    await this.userMenu.click();
    await this.accountSettingsMenuItem.click();
  }

  async logout() {
    await this.userMenu.click();
    await this.logoutMenuItem.click();
    // This is important to make sure that the redirect and subsequent page.goto
    // inside login() don't enter a race condition which results in a
    // net::ERR_ABORTED.
    await expect(this.page.getByText('Sign in to Teleport')).toBeVisible();
  }
}
