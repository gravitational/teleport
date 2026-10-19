/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { useEffect, useRef, type MouseEvent as ReactMouseEvent } from 'react';
import styled from 'styled-components';

import { useStateRef } from 'shared/hooks';

/**
 * A vertical line handle (left edge) that supports mouse dragging to
 * resize a right aligned side panel.
 */
export function SidePanelResizeHandle({
  panelWidth,
  onWidthChange,
}: {
  panelWidth: number;
  onWidthChange(width: number): void;
}) {
  const [isResizing, isResizingRef, setIsResizing] = useStateRef(false);

  const widthRef = useRef(panelWidth);
  const initialMouseX = useRef(0);

  function handleMouseDown(event: ReactMouseEvent) {
    document.body.style.cursor = 'ew-resize';

    event.preventDefault();
    setIsResizing(true);

    widthRef.current = panelWidth;
    initialMouseX.current = event.clientX;
  }

  useEffect(() => {
    function handleMouseMove(event: MouseEvent) {
      if (!isResizingRef.current) {
        return;
      }

      const deltaX = event.clientX - initialMouseX.current;
      const newWidth = widthRef.current - deltaX;

      onWidthChange(newWidth);
    }

    function handleMouseUp() {
      if (!isResizingRef.current) {
        return;
      }

      document.body.style.cursor = '';

      setIsResizing(false);
    }

    window.addEventListener('mousemove', handleMouseMove);
    window.addEventListener('mouseup', handleMouseUp);

    return () => {
      document.body.style.cursor = '';

      window.removeEventListener('mousemove', handleMouseMove);
      window.removeEventListener('mouseup', handleMouseUp);

      setIsResizing(false);
    };
  }, [isResizingRef, onWidthChange, setIsResizing]);

  return (
    <Container onMouseDown={handleMouseDown}>
      <ResizeHandleLine $isResizing={isResizing} />
    </Container>
  );
}

const ResizeHandleLine = styled.div<{ $isResizing: boolean }>`
  position: absolute;
  width: ${p => (p.$isResizing ? 2 : 1)}px;
  background: ${p =>
    p.$isResizing
      ? p.theme.colors.interactive.solid.accent.default
      : p.theme.colors.interactive.tonal.neutral[1]};
  left: 0;
  height: 100%;
`;

const Container = styled.div`
  display: flex;
  justify-content: center;

  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  cursor: ew-resize;
  width: 12px;
  user-select: none;

  // Keep the handle above the Ace editor (e.g. terraform config)
  z-index: 10;

  &:hover ${ResizeHandleLine} {
    background: ${p => p.theme.colors.interactive.solid.accent.default};
    width: 2px;
  }
`;
