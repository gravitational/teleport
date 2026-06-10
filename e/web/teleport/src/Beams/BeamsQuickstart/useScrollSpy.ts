import { useCallback, useEffect, useRef, useState } from 'react';

// Activation line sits in the upper eighth of the viewport so a section
// only "wins" and is displayed once its sticky title has reached the top.
const ACTIVATION_FRACTION = 1 / 8;

// Pixels of slack when detecting scrolled to the bottom.
const BOTTOM_THRESHOLD_PX = 4;

/**
 * `useScrollSpy` returns the element ID (from the given element IDs) of the
 * element nearest the top of the viewport based on scroll position.
 */
export function useScrollSpy<T extends string>(ids: T[]): T | undefined {
  const [active, setActive] = useState<T | undefined>(ids[0]);

  // Refs let the listeners stay attached for the hook's lifetime — they
  // always read the latest values rather than re-attaching when ids
  // changes.
  const idsRef = useRef(ids);
  idsRef.current = ids;
  const scrollerRef = useRef<HTMLElement | null>(null);

  const recompute = useCallback(() => {
    const ids = idsRef.current;
    if (!scrollerRef.current) {
      const lastId = ids[ids.length - 1];
      const lastEl = lastId ? document.getElementById(lastId) : null;
      scrollerRef.current = findScroller(lastEl);
    }
    const scroller = scrollerRef.current;

    if (
      scroller &&
      scroller.scrollTop + scroller.clientHeight >=
        scroller.scrollHeight - BOTTOM_THRESHOLD_PX
    ) {
      setActive(ids[ids.length - 1]);
      return;
    }

    const line = window.innerHeight * ACTIVATION_FRACTION;
    const current = ids.findLast(id => {
      const el = document.getElementById(id);
      return !!el && el.getBoundingClientRect().top <= line;
    });
    setActive(current ?? ids[0]);
  }, []);

  useEffect(() => {
    recompute();
  }, [recompute]);

  useEffect(() => {
    document.addEventListener('scroll', recompute, {
      capture: true,
      passive: true,
    });
    return () =>
      document.removeEventListener('scroll', recompute, { capture: true });
  }, [recompute]);

  useEffect(() => {
    window.addEventListener('resize', recompute);
    return () => window.removeEventListener('resize', recompute);
  }, [recompute]);

  return active;
}

// Walk up the tree to find the nearest scrollable ancestor. Teleport's
// content shell scrolls in an inner `overflow-y: auto` container, not
// `window`, so we can't just read window.scrollY.
function findScroller(el: Element | null): HTMLElement | null {
  for (let cur = el?.parentElement; cur; cur = cur.parentElement) {
    const overflow = getComputedStyle(cur).overflowY;
    if (
      overflow === 'auto' ||
      overflow === 'scroll' ||
      overflow === 'overlay'
    ) {
      return cur;
    }
  }
  return null;
}
