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

import { TshAbortController } from './types';

/**
 * Creates a version of AbortController that can be passed through Electron contextBridge
 */
export default function createAbortController(): TshAbortController {
  const emitter = new EventTarget();

  //TODO(gzdunek): this should be an object version of AbortSignal
  const signal: AbortSignal = {
    aborted: false,
    // TODO(ravicious): Consider aligning the interface of TshAbortSignal with the interface of
    // browser's AbortSignal so that those two can be used interchangeably, for example in the wait
    // function from the shared package.
    //
    // TshAbortSignal doesn't accept the event name as the first argument.
    //
    // TshAbortSignal still needs to have some kind of a unique property so that Connect functions
    // can enforce on a type level that they can only accept TshAbortSignal. Regular abort signals
    // won't work in Connect since abort signals are often passed through the context bridge.
    addEventListener(string, cb: (...args: any[]) => void) {
      emitter.addEventListener('abort', cb);
    },

    removeEventListener(string, cb: (...args: any[]) => void) {
      emitter.removeEventListener('abort', cb);
    },
    throwIfAborted() {},
    onabort: () => {},
    reason: '',
    removeAllListeners() {
      emitter.removeAllListeners();
    },
    eventListeners(eventName) {
      return emitter.eventListeners(eventName);
    },
    dispatchEvent(event: Event) {
      return emitter.dispatchEvent(event);
    },
  };

  return {
    signal,
    abort() {
      // Once abort() has been called and the signal becomes aborted, it cannot be reused.
      // https://dom.spec.whatwg.org/#abortsignal-signal-abort
      if (signal.aborted) {
        return;
      }

      // signal.aborted = true;
      emitter.dispatchEvent(new Event('abort'));
    },
  };
}
