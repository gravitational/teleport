import { useEffect, useState } from 'react';

const SHOW_HINT_TIMEOUT = 1000 * 60 * 5; // 5 minutes

export function useShowHint(enabled: boolean) {
  const [showHint, setShowHint] = useState(false);

  useEffect(() => {
    if (enabled) {
      const id = window.setTimeout(() => setShowHint(true), SHOW_HINT_TIMEOUT);

      return () => window.clearTimeout(id);
    }
  }, [enabled]);

  return showHint;
}
