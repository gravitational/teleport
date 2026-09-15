import { expect } from '@gravitational/e2e/helpers/test';
import type { Page } from '@playwright/test';

export class HeadlessAuthDialogPage {
  private readonly dialog;

  constructor(page: Page) {
    this.dialog = page
      .getByRole('dialog')
      .filter({ hasText: 'Headless authentication' });
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
