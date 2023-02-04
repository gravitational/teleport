import { useEffect, useRef, useState } from 'react';

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
