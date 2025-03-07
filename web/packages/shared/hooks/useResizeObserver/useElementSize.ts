import { useState } from 'react';

import { useResizeObserver } from './';

/**
 * useElementSize returns a ref and the size of the element.
 * The size is updated whenever the element is resized.
 */
export const useElementSize = <T extends HTMLElement = HTMLElement>(
  opts: Parameters<typeof useResizeObserver>[1] = {}
) => {
  const [size, setSize] = useState({ width: 0, height: 0 });
  const ref = useResizeObserver<T>(entry => {
    const { width, height } = entry.contentRect;
    setSize({ width, height });
  }, opts);
  return [ref, size] as const;
};
