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

type SupportedMergeTypes = string | number | Record<string, unknown>;
type MergeTarget = Record<string, unknown> | Array<SupportedMergeTypes>;

export function mergeDeep(target: MergeTarget, ...sources: Array<MergeTarget>) {
  const isObject = obj => obj && typeof obj === 'object' && !Array.isArray(obj);

  const mergeArray = (target, source) => {
    source.forEach((value, index) => {
      if (
        Array.isArray(value) ||
        (isObject(value) && isObject(source[index]))
      ) {
        if (target[index] === undefined) {
          target[index] = source[index];
        } else {
          mergeDeep(target[index], source[index]);
        }
      } else {
        target[index] = source[index];
      }
    });
  };

  if (!sources.length) return target;
  const source = sources.shift();

  if (isObject(target) && isObject(source)) {
    for (const key in source) {
      if (isObject(source[key])) {
        if (!target[key]) Object.assign(target, { [key]: {} });
        mergeDeep(target[key], source[key]);
      } else if (Array.isArray(source[key])) {
        mergeArray(target[key], source[key]);
      } else {
        Object.assign(target, { [key]: source[key] });
      }
    }
  } else if (Array.isArray(target) && Array.isArray(source)) {
    mergeArray(target, source);
  }

  return mergeDeep(target, ...sources);
}

/** Recursively compares two arrays. */
export function arrayObjectIsEqual(
  arr1: unknown[],
  arr2: unknown[],
  options?: {
    /**
     * If `true`, treats fields set to `undefined` as equal to fields that
     * don't exist at all. Doesn't apply to the array itself, but recursively
     * to its elements.
     */
    ignoreUndefined?: boolean;
  }
): boolean {
  return (
    arr1.length === arr2.length &&
    arr1.every((obj, idx) => equalsDeep(obj, arr2[idx], options))
  );
}

/** Recursively compares two values. */
export function equalsDeep(
  val1: unknown,
  val2: unknown,
  options?: {
    /**
     * If `true`, treats fields set to `undefined` as equal to fields that
     * don't exist at all.
     */
    ignoreUndefined?: boolean;
  }
) {
  if (!isObject(val1) || !isObject(val2)) {
    return val1 === val2;
  }

  if (Array.isArray(val1) && Array.isArray(val2)) {
    return arrayObjectIsEqual(val1, val2, options);
  }

  const obj1 = options?.ignoreUndefined ? onlyDefined(val1) : val1;
  const obj2 = options?.ignoreUndefined ? onlyDefined(val2) : val2;

  if (Object.keys(obj1).length !== Object.keys(obj2).length) {
    return false;
  }
  return Object.keys(obj1).every(key => {
    // This check prevents a false positive where lengths of objects are the
    // same, because there are equal numbers of undefined fields on both
    // compared objects, but it just so happens that only differences are
    // between undefined and missing fields.
    if (!Object.hasOwn(obj2, key)) {
      return false;
    }
    return equalsDeep(obj1[key], obj2[key], options);
  });
}

/** Returns an object with undefined fields filtered out. */
function onlyDefined(obj: object): object {
  return Object.fromEntries(
    Object.entries(obj).filter(([, value]) => value !== undefined)
  );
}

export function isInteger(checkVal: any): boolean {
  return Number.isInteger(checkVal) || checkVal == parseInt(checkVal);
}

export function isObject(checkVal: unknown): checkVal is object {
  const type = typeof checkVal;
  return checkVal != null && (type == 'object' || type == 'function');
}

/**
 * Lodash <https://lodash.com/>
 * Copyright JS Foundation and other contributors <https://js.foundation/>
 * Released under MIT license <https://lodash.com/license>
 * Based on Underscore.js 1.8.3 <http://underscorejs.org/LICENSE>
 * Copyright Jeremy Ashkenas, DocumentCloud and Investigative Reporters & Editors
 */
export function runOnce<T extends (...args) => any>(func: T) {
  let n = 2;
  let result;
  return function () {
    if (--n > 0) {
      // This implementation does not pass strictBindCallApply check.
      result = func.apply(this, arguments as any);
    }
    if (n <= 1) {
      func = undefined;
    }
    return result;
  };
}

interface ThrottleSettings {
  leading?: boolean | undefined;
  trailing?: boolean | undefined;
}

/**
 * Lodash <https://lodash.com/>
 * Copyright JS Foundation and other contributors <https://js.foundation/>
 * Released under MIT license <https://lodash.com/license>
 * Based on Underscore.js 1.8.3 <http://underscorejs.org/LICENSE>
 * Copyright Jeremy Ashkenas, DocumentCloud and Investigative Reporters & Editors
 */
export function throttle<T extends (...args: any) => any>(
  func: T,
  wait = 0,
  options?: ThrottleSettings
): DebouncedFunc<T> {
  var leading = true,
    trailing = true;

  if (isObject(options)) {
    leading = 'leading' in options ? !!options.leading : leading;
    trailing = 'trailing' in options ? !!options.trailing : trailing;
  }
  return debounce(func, wait, {
    leading: leading,
    maxWait: wait,
    trailing: trailing,
  });
}

export type DebouncedFunc<T extends (...args: any[]) => any> = {
  (...args: Parameters<T>): ReturnType<T> | undefined;
  cancel(): void;
  flush(): ReturnType<T> | undefined;
};

type DebounceSettings = {
  leading?: boolean | undefined;
  maxWait?: number | undefined;
  trailing?: boolean | undefined;
};

/**
 * Lodash <https://lodash.com/>
 * Copyright JS Foundation and other contributors <https://js.foundation/>
 * Released under MIT license <https://lodash.com/license>
 * Based on Underscore.js 1.8.3 <http://underscorejs.org/LICENSE>
 * Copyright Jeremy Ashkenas, DocumentCloud and Investigative Reporters & Editors
 */
export function debounce<T extends (...args: any) => any>(
  func: T,
  wait = 0,
  options?: DebounceSettings
): DebouncedFunc<T> {
  var lastArgs,
    lastThis,
    maxWait,
    result,
    timerId,
    lastCallTime,
    lastInvokeTime = 0,
    leading = false,
    maxing = false,
    trailing = true;

  if (isObject(options)) {
    leading = !!options.leading;
    maxing = 'maxWait' in options;
    maxWait = maxing ? Math.max(options.maxWait || 0, wait) : maxWait;
    trailing = 'trailing' in options ? !!options.trailing : trailing;
  }

  function invokeFunc(time) {
    var args = lastArgs,
      thisArg = lastThis;

    lastArgs = lastThis = undefined;
    lastInvokeTime = time;
    result = func.apply(thisArg, args);
    return result;
  }

  function leadingEdge(time) {
    // Reset any `maxWait` timer.
    lastInvokeTime = time;
    // Start the timer for the trailing edge.
    timerId = setTimeout(timerExpired, wait);
    // Invoke the leading edge.
    return leading ? invokeFunc(time) : result;
  }

  function remainingWait(time) {
    var timeSinceLastCall = time - lastCallTime,
      timeSinceLastInvoke = time - lastInvokeTime,
      timeWaiting = wait - timeSinceLastCall;

    return maxing
      ? Math.min(timeWaiting, maxWait - timeSinceLastInvoke)
      : timeWaiting;
  }

  function shouldInvoke(time) {
    var timeSinceLastCall = time - lastCallTime,
      timeSinceLastInvoke = time - lastInvokeTime;

    // Either this is the first call, activity has stopped and we're at the
    // trailing edge, the system time has gone backwards and we're treating
    // it as the trailing edge, or we've hit the `maxWait` limit.
    return (
      lastCallTime === undefined ||
      timeSinceLastCall >= wait ||
      timeSinceLastCall < 0 ||
      (maxing && timeSinceLastInvoke >= maxWait)
    );
  }

  function timerExpired() {
    var time = Date.now();
    if (shouldInvoke(time)) {
      return trailingEdge(time);
    }
    // Restart the timer.
    timerId = setTimeout(timerExpired, remainingWait(time));
  }

  function trailingEdge(time) {
    timerId = undefined;

    // Only invoke if we have `lastArgs` which means `func` has been
    // debounced at least once.
    if (trailing && lastArgs) {
      return invokeFunc(time);
    }
    lastArgs = lastThis = undefined;
    return result;
  }

  function cancel() {
    if (timerId !== undefined) {
      clearTimeout(timerId);
    }
    lastInvokeTime = 0;
    lastArgs = lastCallTime = lastThis = timerId = undefined;
  }

  function flush() {
    return timerId === undefined ? result : trailingEdge(Date.now());
  }

  function debounced() {
    var time = Date.now(),
      isInvoking = shouldInvoke(time);

    lastArgs = arguments;
    lastThis = this;
    lastCallTime = time;

    if (isInvoking) {
      if (timerId === undefined) {
        return leadingEdge(lastCallTime);
      }
      if (maxing) {
        // Handle invocations in a tight loop.
        timerId = setTimeout(timerExpired, wait);
        return invokeFunc(lastCallTime);
      }
    }
    if (timerId === undefined) {
      timerId = setTimeout(timerExpired, wait);
    }
    return result;
  }

  debounced.cancel = cancel;
  debounced.flush = flush;
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

/**
 * Lodash <https://lodash.com/>
 * Copyright JS Foundation and other contributors <https://js.foundation/>
 * Released under MIT license <https://lodash.com/license>
 * Based on Underscore.js 1.8.3 <http://underscorejs.org/LICENSE>
 * Copyright Jeremy Ashkenas, DocumentCloud and Investigative Reporters & Editors
 */
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
    // `as any` because the implementation does not pass strictBindCallApply check.
    const result = func.apply(this, args as any);
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

function mapCacheClear() {
  this.size = 0;
  this.__data__ = {
    hash: new Hash(),
    map: new Map(),
    string: new Hash(),
  };
}

function mapCacheDelete(key) {
  var result = getMapData(this, key)['delete'](key);
  this.size -= result ? 1 : 0;
  return result;
}

function mapCacheGet(key) {
  return getMapData(this, key).get(key);
}

function mapCacheHas(key) {
  return getMapData(this, key).has(key);
}

function mapCacheSet(key, value) {
  var data = getMapData(this, key),
    size = data.size;

  data.set(key, value);
  this.size += data.size == size ? 0 : 1;
  return this;
}

MapCache.prototype.clear = mapCacheClear;
MapCache.prototype['delete'] = mapCacheDelete;
MapCache.prototype.get = mapCacheGet;
MapCache.prototype.has = mapCacheHas;
MapCache.prototype.set = mapCacheSet;

function Hash(entries?) {
  var index = -1,
    length = entries == null ? 0 : entries.length;

  this.clear();
  while (++index < length) {
    var entry = entries[index];
    this.set(entry[0], entry[1]);
  }
}

const HASH_UNDEFINED = '__lodash_hash_undefined__';

function hashClear() {
  this.__data__ = Object.create ? Object.create(null) : {};
  this.size = 0;
}

function hashDelete(key) {
  var result = this.has(key) && delete this.__data__[key];
  this.size -= result ? 1 : 0;
  return result;
}

function hashGet(key) {
  var data = this.__data__;
  if (Object.create) {
    var result = data[key];
    return result === HASH_UNDEFINED ? undefined : result;
  }
  return Object.hasOwnProperty.call(data, key) ? data[key] : undefined;
}

function hashHas(key) {
  var data = this.__data__;
  return Object.create
    ? data[key] !== undefined
    : Object.hasOwnProperty.call(data, key);
}

function hashSet(key, value) {
  var data = this.__data__;
  this.size += this.has(key) ? 0 : 1;
  data[key] = Object.create && value === undefined ? HASH_UNDEFINED : value;
  return this;
}

// Add methods to `Hash`.
Hash.prototype.clear = hashClear;
Hash.prototype['delete'] = hashDelete;
Hash.prototype.get = hashGet;
Hash.prototype.has = hashHas;
Hash.prototype.set = hashSet;

function getMapData(map, key) {
  var data = map.__data__;
  return isKeyable(key)
    ? data[typeof key == 'string' ? 'string' : 'hash']
    : data.map;
}

function isKeyable(value) {
  var type = typeof value;
  return type == 'string' ||
    type == 'number' ||
    type == 'symbol' ||
    type == 'boolean'
    ? value !== '__proto__'
    : value === null;
}
