import { type MouseEvent as ReactMouseEvent } from 'react';
import styled from 'styled-components';

import { Box, Flex } from 'design';

import { defaultSidePanelWidth } from '../Shared';

/**
 * PanelResizer renders a draggable vertical border that lets the user resize
 * the side panel. Dragging left widens the panel and dragging right narrows it,
 * clamped between `defaultSidePanelWidth` and 600px. The resize handle is
 * invisible until hovered.
 */
export function PanelResizer({
  panelWidth,
  updatePanelWidth,
}: {
  panelWidth: number;
  updatePanelWidth(width: number): void;
}) {
  function handleMouseDown(e: ReactMouseEvent) {
    e.preventDefault();
    const startMouseX = e.clientX;
    const startPanelWidth = panelWidth;

    // Calculate new panel width as user drags to resize panel.
    function handleMouseMove(e: MouseEvent) {
      const mouseDragged = e.clientX - startMouseX;
      const newWidth = startPanelWidth - mouseDragged;

      if (newWidth >= defaultSidePanelWidth && newWidth <= 900) {
        updatePanelWidth(newWidth);
        return;
      }
    }

    // Clean up after user is done dragging.
    function handleMouseUp() {
      document.removeEventListener('mousemove', handleMouseMove, {
        capture: true,
      });
      document.removeEventListener('mouseup', handleMouseUp, { capture: true });
    }

    document.addEventListener('mousemove', handleMouseMove, { capture: true });
    document.addEventListener('mouseup', handleMouseUp, { capture: true });
  }

  return (
    <Flex
      onMouseDown={handleMouseDown}
      css={`
        z-index: 10000;
      `}
    >
      <ResizeOnHoverBorder />
    </Flex>
  );
}

const ResizeOnHoverBorder = styled(Box)`
  cursor: col-resize;
  height: 100%;
  border-left: 1px solid transparent;
  width: 12px;
  background-color: transparent;
  position: absolute;
  opacity: 0;
  transition: opacity 0.2s ease-in-out;

  &:hover {
    opacity: 1;
    border-left: 1px solid
      ${p => p.theme.colors.interactive.solid.accent.default};
  }
`;
