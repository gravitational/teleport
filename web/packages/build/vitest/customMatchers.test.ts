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

import { setImmediate } from 'node:timers';

import { toEventuallyBeTrue } from './customMatchers';

describe('toEventuallyBeTrue custom matcher', () => {
  it('marks the test as passed if the condition resolves to true within the timeout', async () => {
    let returnValue = false;

    setImmediate(() => {
      returnValue = true;
    });

    const condition = () => returnValue;

    await expect(condition).toEventuallyBeTrue({
      waitFor: 250,
      tick: 5,
    });
  });
});

describe('toEventuallyBeTrue', () => {
  it('rejects if the condition does not resolve to true within the timeout', async () => {
    const condition = () => false;

    await expect(
      // Short waiting time on this test is fine, as we expect toEventuallyBeTrue to return false,
      // so we want it to fail ASAP.
      toEventuallyBeTrue(condition, { waitFor: 5, tick: 3 })
    ).rejects.toBeUndefined();
  });

  it('returns early before scheduling the first tick if the condition is true', async () => {
    const condition = () => true;

    const eventuallyPromise = toEventuallyBeTrue(condition, {
      waitFor: 250,
      tick: 100,
    });
    const timeoutPromise = new Promise((resolve, reject) => {
      setTimeout(() => reject('timeout'), 5);
    });

    // We expect that eventuallyPromise will resolve first. To accomplish this, toEventuallyBeTrue
    // needs to evaluate the condition before scheduling the first tick, as the tick is scheduled to
    // be in 100ms while timeoutPromise is going to reject in 5ms.
    await expect(
      Promise.race([eventuallyPromise, timeoutPromise])
    ).resolves.toBeUndefined();
  });
});
