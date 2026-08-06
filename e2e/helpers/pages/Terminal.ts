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
  private readonly input;
  private readonly terminal;

  constructor(private page: Page) {
    this.input = page.getByRole('textbox', { name: 'Terminal input' });
    this.terminal = page.getByTestId('terminal');
  }

  async waitForReady() {
    await expect(this.input).toBeVisible({ timeout: TERMINAL_TIMEOUT });
  }

  async exec(command: string) {
    await this.input.pressSequentially(command + '\n');
  }

  async waitForText(text: string) {
    await expect(this.terminal).toContainText(text, {
      timeout: TERMINAL_TIMEOUT,
    });
  }

  /**
   * Selects all of the text rendered in the terminal and copies it.
   */
  async copyAllText() {
    await this.page.evaluate(() => {
      const xterm = document.querySelector('.xterm');
      if (!xterm) {
        throw new Error('xterm element not found');
      }

      // Blur the active element to avoid copying from the hidden textarea xterm creates for capturing keystrokes
      (document.activeElement as HTMLElement | null)?.blur();

      const selection = window.getSelection();
      if (!selection) {
        throw new Error('window.getSelection() returned null');
      }

      const range = document.createRange();
      range.selectNodeContents(xterm);
      selection.removeAllRanges();
      selection.addRange(range);
    });

    await this.page.keyboard.press('ControlOrMeta+C');
  }

  async writeClipboard(text: string) {
    await this.page.evaluate(
      value => navigator.clipboard.writeText(value),
      text
    );
  }

  async readClipboard(): Promise<string> {
    return this.page.evaluate(() => navigator.clipboard.readText());
  }
}
