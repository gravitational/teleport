import { DependencyList, MutableRefObject, useEffect, useRef } from 'react';

/**
 * Returns `ref` object that is automatically focused when `shouldFocus` is `true`.
 * Focus can be also re triggered by changing any of the `refocusDeps`.
 */
export function useRefAutoFocus<T extends { focus(): void }>(options: {
  shouldFocus: boolean;
  refocusDeps?: DependencyList;
}): MutableRefObject<T> {
  const ref = useRef<T>();

  useEffect(() => {
    if (options.shouldFocus) {
      ref.current?.focus();
    }
  }, [options.shouldFocus, ref, ...(options.refocusDeps || [])]);

  return ref;
}
