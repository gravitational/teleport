import { expect, type Locator, type Page } from '@playwright/test';

import { CLUSTER_NAME } from '../test';

export type RecordingType = 'ssh' | 'k8s' | 'desktop' | 'database';

export class PlayerPage {
  readonly terminal: Locator;

  constructor(protected page: Page) {
    this.terminal = page.locator('.xterm');
  }

  async goto(sessionId: string, recordingType: RecordingType) {
    await this.page.goto(
      `/web/cluster/${CLUSTER_NAME}/session/${sessionId}?recordingType=${recordingType}&durationMs=1000`
    );
  }

  async expectError(text: string | RegExp) {
    await expect(this.page.getByText(text)).toBeVisible();
  }

  getByText(text: string) {
    return this.page.getByText(text);
  }
}
