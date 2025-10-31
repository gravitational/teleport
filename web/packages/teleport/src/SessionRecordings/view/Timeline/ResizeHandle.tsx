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

import {
  useCallback,
  useEffect,
  useRef,
  type MouseEvent as ReactMouseEvent,
} from 'react';
import styled from 'styled-components';

import { useStateRef } from 'shared/hooks';

const ResizeHandleLine = styled.div<{ active: boolean }>`
  position: absolute;
  height: ${p => (p.active ? 2 : 1)}px;
  background: ${p =>
    p.active
      ? p.theme.colors.sessionRecordingTimeline.border.hover
      : p.theme.colors.sessionRecordingTimeline.border.default};
  top: 50%;
  left: 0;
  right: 0;
  z-index: 1;
`;

const Container = styled.div`
  --height: 14px;
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  cursor: ns-resize;
  height: var(--height);
  user-select: none;
  z-index: 2;

  &:hover ${ResizeHandleLine} {
    background: ${p => p.theme.colors.sessionRecordingTimeline.border.hover};
    height: 2px;
  }
`;

interface ResizeHandleProps {
  onChange: (newHeight: number) => void;
  height: number;
  defaultHeight: number;
  minHeight: number;
  maxHeight: number;
}

export function ResizeHandle({
  onChange,
  height,
  defaultHeight,
  minHeight,
  maxHeight,
}: ResizeHandleProps) {
  const [isResizing, isResizingRef, setIsResizing] = useStateRef(false);

  const heightRef = useRef(defaultHeight);
  const initialMouseY = useRef<number>(0);

  const handleMouseDown = useCallback(
    (e: ReactMouseEvent<HTMLDivElement>) => {
      document.body.style.cursor = 'ns-resize';

      e.preventDefault();
      e.stopPropagation();

      setIsResizing(true);

      heightRef.current = height;
      initialMouseY.current = e.clientY;
    },
    [height, setIsResizing]
  );

  const handleDoubleClick = useCallback(() => {
    document.body.style.cursor = 'default';

    onChange(defaultHeight);
  }, [onChange, defaultHeight]);

  useEffect(() => {
    function handleMouseMove(event: MouseEvent) {
      if (!isResizingRef.current) {
        return;
      }

      const deltaY = initialMouseY.current - event.clientY;
      const newHeight = heightRef.current + deltaY;

      const clampedHeight = Math.max(minHeight, Math.min(maxHeight, newHeight));

      onChange(clampedHeight);
    }

    function handleMouseUp() {
      if (!isResizingRef.current) {
        return;
      }

      document.body.style.cursor = 'default';

      window.setTimeout(() => {
        setIsResizing(false);
      }, 100);
    }

    window.addEventListener('mousemove', handleMouseMove);
    window.addEventListener('mouseup', handleMouseUp);

    return () => {
      document.body.style.cursor = 'default';

      window.removeEventListener('mousemove', handleMouseMove);
      window.removeEventListener('mouseup', handleMouseUp);

      setIsResizing(false);
    };
  }, [isResizingRef, maxHeight, minHeight, onChange, setIsResizing]);

  return (
    <Container
      onMouseDown={handleMouseDown}
      onDoubleClick={handleDoubleClick}
      style={{
        transform: `translateY(calc(${height * -1}px + var(--height) / 2))`,
      }}
    >
      <ResizeHandleLine active={isResizing} />
    </Container>
  );
}
