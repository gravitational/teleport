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

import createAbortController from './createAbortController';

test('abort controller emits the abort event only once', () => {
  const listener = jest.fn();
  const controller = createAbortController();

  controller.signal.addEventListener(listener);
  controller.abort();

  // This makes sure that the implementation doesn't depend solely on `emitter.once('abort', cb)` to
  // implement this. Once a signal has been aborted, its state changes and it cannot be reused.
  //
  // This mirrors the browser implementation.
  const listenerAddedAfterAbort = jest.fn();
  controller.signal.addEventListener(listenerAddedAfterAbort);
  controller.abort();

  expect(listener).toHaveBeenCalledTimes(1);
  expect(listenerAddedAfterAbort).not.toHaveBeenCalled();
});

test('abort updates signal.aborted', () => {
  const controller = createAbortController();
  expect(controller.signal.aborted).toBe(false);

  controller.abort();
  expect(controller.signal.aborted).toBe(true);
});
