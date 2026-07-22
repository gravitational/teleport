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

export const DEFAULT_SIDEBAR_WIDTH = 350;
export const MIN_SIDEBAR_WIDTH = 250;
export const MAX_SIDEBAR_WIDTH = 600;

interface SidebarResizeHandleProps {
  onChange: (newWidth: number) => void;
  width: number;
  defaultWidth?: number;
}

export function SidebarResizeHandle({
  onChange,
  width,
  defaultWidth,
}: SidebarResizeHandleProps) {
  const [isResizing, isResizingRef, setIsResizing] = useStateRef(false);

  const defaultSidebarWidth = defaultWidth ?? DEFAULT_SIDEBAR_WIDTH;

  const widthRef = useRef(defaultSidebarWidth);
  const initialMouseX = useRef<number>(0);

  const handleMouseDown = useCallback(
    (e: ReactMouseEvent<HTMLDivElement>) => {
      document.body.style.cursor = 'ew-resize';

      e.preventDefault();
      e.stopPropagation();

      setIsResizing(true);

      widthRef.current = width;
      initialMouseX.current = e.clientX;
    },
    [width, setIsResizing]
  );

  const handleDoubleClick = useCallback(() => {
    document.body.style.cursor = 'default';

    onChange(defaultSidebarWidth);
  }, [onChange]);

  useEffect(() => {
    function handleMouseMove(event: MouseEvent) {
      if (!isResizingRef.current) {
        return;
      }

      const deltaX = event.clientX - initialMouseX.current;
      const newWidth = widthRef.current + deltaX;

      const clampedWidth = Math.max(
        MIN_SIDEBAR_WIDTH,
        Math.min(MAX_SIDEBAR_WIDTH, newWidth)
      );

      onChange(clampedWidth);
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
  }, [isResizingRef, onChange, setIsResizing]);

  return (
    <Container
      onMouseDown={handleMouseDown}
      onDoubleClick={handleDoubleClick}
      style={{
        transform: `translateX(calc(var(--width) / 2))`,
      }}
    >
      <ResizeHandleLine active={isResizing} />
    </Container>
  );
}

const ResizeHandleLine = styled.div<{ active: boolean }>`
  position: absolute;
  width: ${p => (p.active ? 2 : 1)}px;
  background: ${p =>
    p.active
      ? p.theme.colors.sessionRecordingTimeline.border.hover
      : p.theme.colors.sessionRecordingTimeline.border.default};
  right: 50%;
  top: 0;
  bottom: 0;
  z-index: 1;
`;

const Container = styled.div`
  --width: 14px;
  position: absolute;
  top: 0;
  bottom: 0;
  right: 0;
  cursor: ew-resize;
  width: var(--width);
  user-select: none;
  z-index: 2;

  &:hover ${ResizeHandleLine} {
    background: ${p => p.theme.colors.sessionRecordingTimeline.border.hover};
    width: 2px;
  }
`;
