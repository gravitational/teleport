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
