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

export function isInteger(checkVal: any): boolean {
  return Number.isInteger(checkVal) || checkVal == parseInt(checkVal);
}

export function isObject(checkVal) {
  const type = typeof checkVal;
  return checkVal != null && (type == 'object' || type == 'function');
}

// Lift & Shift from lodash
export function runOnce(func) {
  let n = 2;
  let result;
  return function () {
    if (--n > 0) {
      result = func.apply(this, arguments);
    }
    if (n <= 1) {
      func = undefined;
    }
    return result;
  };
}

export type DebouncedFunc<T extends (...args: any[]) => any> = {
  (...args: Parameters<T>): ReturnType<T> | undefined;
  cancel(): void;
};

// Lift & Shift from lodash
export function debounce<T extends (...args: any) => any>(
  func: T,
  wait: number | undefined
): DebouncedFunc<T> {
  let lastArgs, lastThis, result, timerId, lastCallTime;

  function invokeFunc() {
    const args = lastArgs;
    const thisArg = lastThis;

    lastArgs = lastThis = undefined;
    result = func.apply(thisArg, args);
    return result;
  }

  function remainingWait(time) {
    const timeSinceLastCall = time - lastCallTime;
    const timeWaiting = wait - timeSinceLastCall;
    return timeWaiting;
  }

  function shouldInvoke(time) {
    const timeSinceLastCall = time - lastCallTime;

    // Either this is the first call, activity has stopped and we're at the
    // trailing edge, the system time has gone backwards and we're treating
    // it as the trailing edge, or we've hit the `maxWait` limit.
    return (
      lastCallTime === undefined ||
      timeSinceLastCall >= wait ||
      timeSinceLastCall < 0
    );
  }

  function timerExpired() {
    const time = Date.now();
    if (shouldInvoke(time)) {
      return trailingEdge();
    }
    // Restart the timer.
    timerId = setTimeout(timerExpired, remainingWait(time));
  }

  function trailingEdge() {
    timerId = undefined;

    // Only invoke if we have `lastArgs` which means `func` has been
    // debounced at least once.
    if (lastArgs) {
      return invokeFunc();
    }
    lastArgs = lastThis = undefined;
    return result;
  }

  function cancel() {
    if (timerId !== undefined) {
      clearTimeout(timerId);
    }
    lastArgs = lastCallTime = lastThis = timerId = undefined;
  }

  function debounced() {
    const time = Date.now();
    const isInvoking = shouldInvoke(time);

    lastArgs = arguments;
    lastThis = this;
    lastCallTime = time;

    if (isInvoking) {
      if (timerId === undefined) {
        // Start the timer for the trailing edge.
        timerId = setTimeout(timerExpired, wait);
        return result;
      }
    }
    if (timerId === undefined) {
      timerId = setTimeout(timerExpired, wait);
    }
    return result;
  }

  debounced.cancel = cancel;
  return debounced;
}

interface MapCacheType {
  delete(key: any): boolean;
  get(key: any): any;
  has(key: any): boolean;
  set(key: any, value: any): this;
  clear?: (() => void) | undefined;
}

type MemoizedFunction = {
  cache: MapCacheType;
};

// Lift & Shift from lodash
export function memoize<T extends (...args: any) => any>(
  func: T
): T & MemoizedFunction {
  const memoized = function () {
    const args = arguments;
    const key = args[0];
    const cache = memoized.cache;

    if (cache.has(key)) {
      return cache.get(key);
    }
    const result = func.apply(this, args);
    memoized.cache = cache.set(key, result) || cache;
    return result;
  };
  memoized.cache = new (memoize.Cache || MapCache)();
  /* eslint-disable @typescript-eslint/ban-ts-comment*/
  // @ts-ignore
  return memoized;
}

// Expose `MapCache`.
memoize.Cache = MapCache;

function MapCache(entries?: any) {
  let index = -1;
  const length = entries == null ? 0 : entries.length;

  this.clear();
  while (++index < length) {
    const entry = entries[index];
    this.set(entry[0], entry[1]);
  }
}
