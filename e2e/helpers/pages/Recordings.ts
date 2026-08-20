import type { Page } from '@playwright/test';

import { CLUSTER_NAME } from '../test';

export class RecordingsPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto(`/web/cluster/${CLUSTER_NAME}/recordings`);
  }
}
