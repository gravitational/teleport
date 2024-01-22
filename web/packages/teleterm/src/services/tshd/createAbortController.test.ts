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
