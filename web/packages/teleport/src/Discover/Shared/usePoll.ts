import { useEffect, useRef, useState } from 'react';

export function usePoll<T>(
  callback: (signal: AbortSignal) => Promise<T | null>,
  enabled: boolean,
  interval = 1000
): T | null {
  const abortController = useRef(new AbortController());
  const [result, setResult] = useState<T | null>(null);

  useEffect(() => {
    if (enabled) {
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
    } else {
      setResult(null);
    }
  }, [enabled, interval, callback]);

  return result;
}
