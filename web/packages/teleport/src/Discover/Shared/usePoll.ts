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

import { useEffect, useRef, useState } from 'react';

/**
 * While enabled is true, usePoll runs the callback every interval milliseconds,
 * returning the result of callback once it arrives, and null otherwise.
 *
 * The polling *does not* automatically terminate once a result is found. It is the caller's
 * responsibility to terminate the poll by switching the enabled parameter to false.
 * (This allows for callers to implement more complex logic for terminating polling).
 */
export function usePoll<T>(
  callback: (signal: AbortSignal) => Promise<T | null>,
  enabled: boolean,
  interval = 1000
): T | null {
  const abortController = useRef(new AbortController());
  const [result, setResult] = useState<T | null>(null);
  const [running, setRunning] = useState(false);

  useEffect(() => {
    if (enabled && !running) {
      setResult(null);
      setRunning(true);
    }

    if (!enabled && running) {
      setRunning(false);
    }
  }, [callback, enabled, running]);

  useEffect(() => {
    if (enabled && running) {
      abortController.current = new AbortController();

      const id = window.setInterval(async () => {
        try {
          const result = await callback(abortController.current.signal);

          if (result) {
            clearInterval(id);
            setResult(result);
          }
          // eslint-disable-next-line no-empty
        } catch {}
      }, interval);

      return () => {
        clearInterval(id);
        abortController.current.abort();
      };
    }
  }, [enabled, interval, callback, running]);

  return result;
}
