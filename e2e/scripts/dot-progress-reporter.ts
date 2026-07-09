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

import type { Reporter, TestCase, TestResult } from '@playwright/test/reporter';

const green = (s: string) => `\x1b[32m${s}\x1b[39m`;
const red = (s: string) => `\x1b[31m${s}\x1b[39m`;
const yellow = (s: string) => `\x1b[33m${s}\x1b[39m`;
const gray = (s: string) => `\x1b[90m${s}\x1b[39m`;

/**
 * Prints colored progress dots like the built-in dot reporter but skips
 * the failure summary at the end. The Go runner prints its own summary
 * from the merged JSON report.
 */
class DotProgressReporter implements Reporter {
  private column = 0;

  onTestEnd(test: TestCase, result: TestResult) {
    let char: string;
    switch (result.status) {
      case 'passed':
        char = test.outcome() === 'flaky' ? yellow('\u00b1') : green('\u00b7');
        break;
      case 'failed':
      case 'timedOut':
        char = red(result.status === 'timedOut' ? 'T' : 'F');
        break;
      case 'skipped':
        char = yellow('\u00b0');
        break;
      case 'interrupted':
        char = gray('\u00d7');
        break;
      default:
        char = '?';
    }

    process.stdout.write(char);
    this.column++;

    if (this.column >= 80) {
      process.stdout.write('\n');
      this.column = 0;
    }
  }

  onEnd() {
    if (this.column > 0) {
      process.stdout.write('\n');
    }
  }
}

export default DotProgressReporter;
