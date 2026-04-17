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
    this.terminal = page.locator('.xterm');
  }

  async waitForReady() {
    await expect(this.input).toBeVisible({ timeout: TERMINAL_TIMEOUT });
  }

  async exec(command: string) {
    await this.input.pressSequentially(command + '\n');
  }

  /**
   * Executes a shell command in the terminal and waits for its completion.
   * Appends an exit-code marker so output can be matched to this specific command.
   *
   * @returns The trimmed output of the command.
   * @throws {Error} If the exit code cannot be parsed or is not 0.
   */
  async execAndWait(
    command: string,
    options: { timeout?: number } = {}
  ): Promise<string> {
    // Generate a unique marker per command
    const marker = `__EXIT_${crypto.randomUUID().split('-').at(0)}__=`;
    const fullCommand = `${command}; echo "${marker}$?"`;
    const terminalRows = this.terminal.locator('.xterm-rows');

    await this.input.pressSequentially(`${fullCommand}\n`);

    // Wait for the marker plus the exit code
    const markerWithExitCodeRegex = new RegExp(`${marker}\\d+`);
    await expect(terminalRows).toContainText(markerWithExitCodeRegex, {
      timeout: options?.timeout || TERMINAL_TIMEOUT,
    });

    const text = (await terminalRows.textContent()) || '';

    const fullCommandIndex = text.lastIndexOf(fullCommand);
    const markerIndex = text.lastIndexOf(marker);

    // Extract output between the typed command and the exit marker
    const output = text
      .slice(fullCommandIndex + fullCommand.length, markerIndex)
      .trim();

    // Extract the exit code immediately following the marker
    const exitCodeMatch = text
      .slice(markerIndex + marker.length)
      .match(/^(\d+)/);
    if (!exitCodeMatch) {
      throw new Error(`Could not find exit code, output: ${output}`);
    }

    const exitCode = Number(exitCodeMatch[1]);

    if (exitCode !== 0) {
      throw new Error(
        `Command failed with ${exitCode} exit code, output: ${output}`
      );
    }

    return output;
  }

  async waitForText(text: string) {
    await expect(this.terminal).toContainText(text, {
      timeout: TERMINAL_TIMEOUT,
    });
  }
}
