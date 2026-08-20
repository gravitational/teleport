import type { Locator, Page } from '@playwright/test';

export class TrustedClustersPage {
  readonly connectButton: Locator;
  readonly editButton: Locator;
  readonly saveChangesButton: Locator;
  readonly editor: Locator;

  constructor(private page: Page) {
    this.connectButton = page.getByRole('button', {
      name: 'Connect to Root Cluster',
    });

    this.editButton = page.getByRole('button', {
      name: 'Edit Trusted Cluster',
    });

    this.saveChangesButton = page.getByRole('button', {
      name: 'Save changes',
    });

    this.editor = page.locator('.ace_editor');
  }

  async goto() {
    await this.page.goto('/web/trust');
  }

  async appendToYamlAndSave() {
    await this.editButton.click();

    await this.editor.waitFor({ state: 'visible' });

    await this.page.evaluate(() => {
      const editor = (window as any).ace.edit(
        document.querySelector('.ace_editor')
      );

      editor.session.setValue(editor.session.getValue() + '\n');
    });

    await this.saveChangesButton.click();
  }
}
