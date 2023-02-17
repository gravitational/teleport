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

export function usePoll<T>(
  callback: (signal: AbortSignal) => Promise<T | null>,
  timeout: number,
  enabled: boolean,
  interval = 1000
) {
  const abortController = useRef(new AbortController());

  const [running, setRunning] = useState(false);
  const [timedOut, setTimedOut] = useState(false);
  const [result, setResult] = useState<T | null>(null);

  useEffect(() => {
    if (enabled && !running) {
      setResult(null);
      setTimedOut(false);
      setRunning(true);
    }

    if (!enabled && running) {
      setRunning(false);
    }
  }, [callback, enabled, running]);

  useEffect(() => {
    if (running && timeout > Date.now()) {
      const id = window.setTimeout(() => {
        setTimedOut(true);
      }, timeout - Date.now());

      return () => clearTimeout(id);
    }
  }, [running, timeout]);

  useEffect(() => {
    if (running) {
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
  }, [running, timedOut, interval, callback]);

  return { timedOut, result };
}
