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
import { useEffect, useRef } from 'react';

/** Resolves after a given duration. */
export function wait(ms: number, abortSignal?: AbortSignal): Promise<void> {
  if (abortSignal?.aborted) {
    return Promise.reject(new DOMException('Wait was aborted.', 'AbortError'));
  }

  return new Promise((resolve, reject) => {
    const abort = () => {
      clearTimeout(timeout);
      reject(new DOMException('Wait was aborted.', 'AbortError'));
    };
    const done = () => {
      abortSignal?.removeEventListener('abort', abort);
      resolve();
    };

    const timeout = setTimeout(done, ms);
    abortSignal?.addEventListener('abort', abort, { once: true });
  });
}

/** Blocks until the signal is aborted. */
export function waitForever(abortSignal: AbortSignal): Promise<never> {
  if (abortSignal.aborted) {
    return Promise.reject(new DOMException('Wait was aborted.', 'AbortError'));
  }

  return new Promise((_, reject) => {
    const abort = () => {
      reject(new DOMException('Wait was aborted.', 'AbortError'));
    };

    abortSignal.addEventListener('abort', abort, { once: true });
  });
}

/**
 * usePromiseRejectedOnUnmount is useful when writing stories for loading states.
 */
export const usePromiseRejectedOnUnmount = () => {
  const abortControllerRef = useRef(new AbortController());

  useEffect(() => {
    return () => {
      abortControllerRef.current.abort();
    };
  }, []);

  const promiseRef = useRef<Promise<never>>(undefined);
  if (!promiseRef.current) {
    promiseRef.current = waitForever(abortControllerRef.current.signal);
  }

  return promiseRef.current;
};
