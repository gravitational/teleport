import { expect, Locator, Page } from '@playwright/test';

export class AccountSettingsPage {
  readonly passkeyRows: Locator;
  readonly addPasskeyButton: Locator;
  readonly mfaRows: Locator;

  // Elements of the passkey and MFA device wizards.
  readonly verifyIdentityHeading: Locator;
  readonly verifyMyIdentityButton: Locator;
  readonly createPasskeyButton: Locator;
  readonly passkeyNicknameField: Locator;
  readonly savePasskeyButton: Locator;

  constructor(page: Page) {
    this.passkeyRows = page.getByTestId('passkey-list').locator('tbody tr');
    this.mfaRows = page.getByTestId('mfa-list').locator('tbody tr');
    this.addPasskeyButton = page.getByRole('button', { name: 'Add a Passkey' });
    this.verifyIdentityHeading = page.getByRole('heading', {
      name: 'Verify Identity',
    });
    this.verifyMyIdentityButton = page.getByRole('button', {
      name: 'Verify my identity',
    });
    this.createPasskeyButton = page.getByRole('button', {
      name: 'Create a Passkey',
    });
    this.passkeyNicknameField = page.getByLabel('Passkey Nickname');
    this.savePasskeyButton = page.getByRole('button', {
      name: 'Save the Passkey',
    });
  }

  async addPasskey() {
    await this.addPasskeyButton.click();
  }

  async verifyIdentity() {
    // Sanity check to make sure that the opposite expectation below is actually
    // meaningful.
    await expect(this.verifyIdentityHeading).toBeVisible();
    await this.verifyMyIdentityButton.click();
    // Wait until the ceremony is fully completed and it's OK to switch to the
    // new MFA device.
    await expect(this.verifyIdentityHeading).not.toBeVisible();
  }

  async createPasskey() {
    await this.createPasskeyButton.click();
  }

  async setPasskeyNickname(nickname: string) {
    await this.passkeyNicknameField.fill(nickname);
  }

  async savePasskey() {
    await this.savePasskeyButton.click();
  }
}
