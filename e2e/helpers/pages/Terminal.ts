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

import { expect } from '../test';

const TERMINAL_TIMEOUT = 10_000;

export class TerminalPage {
  private readonly input = this.page.getByRole('textbox', {
    name: 'Terminal input',
  });
  private readonly terminal = this.page.locator('[data-testid="terminal"]');

  constructor(private page: Page) {}

  async waitForReady() {
    await expect(this.input).toBeVisible({ timeout: TERMINAL_TIMEOUT });
  }

  async exec(command: string) {
    await this.input.pressSequentially(command + '\n');
  }

  async expectSnapshot(name: string) {
    await this.page.waitForTimeout(500);
    await expect(this.terminal).toHaveScreenshot(name, {
      timeout: TERMINAL_TIMEOUT,
      maxDiffPixelRatio: 0.02,
    });
  }
}
