/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { useCallback, useRef, type MouseEvent, type RefObject } from 'react';

interface UseCursorOptions {
  containerRef: RefObject<HTMLElement>;
  cursorRef: RefObject<HTMLElement>;
}

const INTERACTION_TIMEOUT = 300;

/**
 * useCursor is a hook that provides mouse event handlers to manage a custom cursor element within a container.
 * It tracks whether the user is interacting with the container and updates the cursor's position accordingly.
 * It returns a ref for accessing the interaction state.
 */
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
