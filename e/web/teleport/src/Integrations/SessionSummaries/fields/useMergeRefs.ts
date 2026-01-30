import { type Ref, type RefCallback } from 'react';

export function useMergeRefs<T>(
  ...refs: (Ref<T> | undefined)[]
): RefCallback<T> {
  return (element: T) => {
    for (const ref of refs) {
      if (!ref) {
        continue;
      }

      if (typeof ref === 'function') {
        ref(element);
      } else {
        ref.current = element;
      }
    }
  };
}
