import { useEffect, useLayoutEffect, useRef, type RefObject } from 'react';

/**
 * useResizeObserver creates a ResizeObserver which fires a callback
 * when the provided element is resized. The callback is called with the
 * ResizeObserverEntry. The observer is disconnected when the element is
 * unmounted.
 *
 * Returns a ref to attach to the element to be observed – the element
 * must be a HTMLElement but may be conditionally null.
 */
export const useResizeObserver = <T extends HTMLElement = HTMLElement>(
  callback: (entry: ResizeObserverEntry) => void,
  { enabled = true, fireOnZeroHeight = true } = {}
): RefObject<T> => {
  const ref = useRef<T>(null);
  const callbackRef = useRef(callback);
  const observerRef = useRef<ResizeObserver | null>(null);

  useEffect(() => {
    callbackRef.current = callback;
  }, [callback]);

  useLayoutEffect(() => {
    if (!enabled) {
      return;
    }

    observerRef.current ||= new ResizeObserver(entries => {
      if (!fireOnZeroHeight && entries[0].contentRect.height === 0) return;
      callbackRef.current(entries[0]);
    });

    const element = ref.current;

    if (element) {
      observerRef.current.observe(element);
    }

    return () => {
      if (observerRef.current && element) {
        observerRef.current.unobserve(element);
      }
    };
  }, [fireOnZeroHeight, enabled]);

  useEffect(() => {
    return () => {
      if (observerRef.current) {
        observerRef.current.disconnect();
        observerRef.current = null;
      }
    };
  }, []);

  return ref;
};
