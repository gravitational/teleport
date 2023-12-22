/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

// Types for custom matchers are defined in jest.d.ts.
// https://jestjs.io/docs/27.x/expect#expectextendmatchers
// https://redd.one/blog/practical-guide-to-custom-jest-matchers
expect.extend({
  /**
   * toEventuallyBeTrue passes the check if condition resolves to true within the timeout passed in
   * waitFor. The condition will be evaluated every tick + the time of evaluation of the condition
   * itself.
   *
   * Default values for waitFor and tick are not provided on purpose, to encourage callsites to
   * decide what values are best suited for their specific use case.
   */
  toEventuallyBeTrue(
    condition: () => boolean,
    args: {
      waitFor: number;
      tick: number;
    }
  ) {
    if (this.isNot) {
      throw new Error(
        'toEventuallyBeTrue was not written with .not in mind; ' +
          'inspect the implementation and verify that it works properly with .not'
      );
    }

    return toEventuallyBeTrue(condition, args).then(
      // Auxiliary then so that the promise above doesn't have to worry about Jest-specific details.
      () => ({
        pass: true,
        message: () => `TODO: .not not implemented`,
      }),
      () => ({
        pass: false,
        message: () =>
          `expected condition to become true within ${args.waitFor}ms`,
      })
    );
  },
});

export const toEventuallyBeTrue = (
  condition: () => boolean,
  {
    waitFor,
    tick,
  }: {
    waitFor: number;
    tick: number;
  }
) => {
  // The promise has two timeouts:
  //   * timer which will reject the promise if the condition doesn't evaluate within waitFor.
  //   * ticker which controls evaluating the condition every tick.
  return new Promise<void>((resolve, reject) => {
    const timer = setTimeout(() => {
      reject();
      clearTimeout(ticker);
    }, waitFor);
    let ticker: NodeJS.Timeout;

    // Use recursion instead of setInterval to ensure that the previous tick has finished before
    // executing the next one.
    // https://developer.mozilla.org/en-US/docs/Web/API/setInterval#ensure_that_execution_duration_is_shorter_than_interval_frequency
    const tickerLoop = () => {
      if (condition()) {
        resolve();
        clearTimeout(timer);
      } else {
        ticker = setTimeout(tickerLoop, tick);
      }
    };

    tickerLoop();
  });
};
