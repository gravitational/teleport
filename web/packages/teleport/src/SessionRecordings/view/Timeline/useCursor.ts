import { useCallback, useRef, type MouseEvent, type RefObject } from 'react';

interface UseCursorOptions {
  containerRef: RefObject<HTMLElement>;
  cursorRef: RefObject<HTMLElement>;
}

const INTERACTION_TIMEOUT = 300;

export function useCursor({ containerRef, cursorRef }: UseCursorOptions) {
  const isInteractingRef = useRef(false);
  const interactionTimeoutRef = useRef<null | number>(null);

  const handleInteractionStart = useCallback(() => {
    isInteractingRef.current = true;

    if (interactionTimeoutRef.current) {
      window.clearTimeout(interactionTimeoutRef.current);
    }
  }, []);

  const handleInteractionEnd = useCallback(() => {
    interactionTimeoutRef.current = window.setTimeout(() => {
      isInteractingRef.current = false;
    }, INTERACTION_TIMEOUT);
  }, []);

  const handleMouseEnter = useCallback(
    (event: MouseEvent) => {
      if (!cursorRef.current || !containerRef.current) {
        return;
      }

      handleInteractionStart();

      const x =
        event.clientX - containerRef.current.getBoundingClientRect().left;

      cursorRef.current.style.display = 'block';
      cursorRef.current.style.transform = `translateX(${x}px) translateZ(0)`;
    },
    [handleInteractionStart, containerRef, cursorRef]
  );

  const handleMouseLeave = useCallback(() => {
    if (!cursorRef.current) {
      return;
    }

    handleInteractionEnd();

    cursorRef.current.style.display = 'none';
  }, [cursorRef, handleInteractionEnd]);

  const handleMouseMove = useCallback(
    (e: MouseEvent) => {
      if (!cursorRef.current || !containerRef.current) {
        return;
      }

      handleInteractionStart();

      const x = e.clientX - containerRef.current.getBoundingClientRect().left;

      cursorRef.current.style.transform = `translateX(${x}px) translateZ(0)`;
    },
    [containerRef, cursorRef, handleInteractionStart]
  );

  return {
    handleMouseEnter,
    handleMouseLeave,
    handleMouseMove,
    isInteractingRef,
  };
}
