import { test as base } from '@gravitational/e2e/helpers/test';

import { PlayerPageE } from './pages/Player';

export const test = base.extend<{ playerPage: PlayerPageE }>({
  playerPage: async ({ page }, use) => {
    await use(new PlayerPageE(page));
  },
});

export { expect } from '@gravitational/e2e/helpers/test';
export type { Locator, Page } from '@gravitational/e2e/helpers/test';
