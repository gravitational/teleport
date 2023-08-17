/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { toEventuallyBeTrue } from './customMatchers';

describe('toEventuallyBeTrue custom matcher', () => {
  it('marks the test as passed if the condition resolves to true within the timeout', async () => {
    let returnValue = false;

    setTimeout(() => {
      returnValue = true;
    }, 7);

    const condition = () => returnValue;

    await expect(condition).toEventuallyBeTrue({
      waitFor: 25,
      tick: 5,
    });
  });
});

describe('toEventuallyBeTrue', () => {
  it('rejects if the condition does not resolve to true within the timeout', async () => {
    const condition = () => false;

    await expect(
      toEventuallyBeTrue(condition, { waitFor: 5, tick: 3 })
    ).rejects.toBeUndefined();
  });

  it('returns early before scheduling the first tick if the condition is true', async () => {
    const condition = () => true;

    const eventuallyPromise = toEventuallyBeTrue(condition, {
      waitFor: 25,
      tick: 10,
    });
    const timeoutPromise = new Promise((resolve, reject) => {
      setTimeout(() => reject('timeout'), 5);
    });

    await expect(
      Promise.race([eventuallyPromise, timeoutPromise])
    ).resolves.toBeUndefined();
  });
});
