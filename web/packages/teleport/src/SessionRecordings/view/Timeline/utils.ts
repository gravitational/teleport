/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
import type { SessionRecordingThumbnail } from 'teleport/services/recordings';

import { LEFT_PADDING } from './constants';

const zoomLevelWithCursor = 2;
const zoomLevelWithoutCursor = 1;

/**
 * calculateThumbnailViewport calculates the portion of the thumbnail image to display
 * based on the cursor position and whether the cursor is visible.
 * It returns the source coordinates and dimensions to be used in drawing the image,
 * as well as the zoom level applied.
 */
export function calculateThumbnailViewport(
  thumbnail: SessionRecordingThumbnail,
  width: number,
  height: number
) {
  // Use different zoom levels based on cursor visibility
  const zoomLevel = thumbnail.cursorVisible
    ? zoomLevelWithCursor
    : zoomLevelWithoutCursor;

  const visibleWidthPercent = 100 / zoomLevel;
  const visibleHeightPercent = 100 / zoomLevel;

  let bgPosX: number;
  let bgPosY: number;

  if (thumbnail.cursorVisible) {
    // Calculate cursor position as percentage
    const cursorPercentX = (thumbnail.cursorX / thumbnail.cols) * 100;
    const cursorPercentY = (thumbnail.cursorY / thumbnail.rows) * 100;

    // Calculate the top-left position percentage to center on cursor
    bgPosX = Math.max(
      0,
      Math.min(
        100 - visibleWidthPercent,
        cursorPercentX - visibleWidthPercent / 2
      )
    );
    bgPosY = Math.max(
      0,
      Math.min(
        100 - visibleHeightPercent,
        cursorPercentY - visibleHeightPercent / 2
      )
    );
  } else {
    // Center the viewport when cursor is not visible
    bgPosX = (100 - visibleWidthPercent) / 2;
    bgPosY = (100 - visibleHeightPercent) / 2;
  }

  // Calculate source dimensions
  const sourceWidth = width / zoomLevel;
  const sourceHeight = height / zoomLevel;

  // Convert percentages to pixel coordinates on the source image
  const maxSourceX = width - sourceWidth;
  const maxSourceY = height - sourceHeight;

  const sourceX = (bgPosX / (100 - visibleWidthPercent)) * maxSourceX || 0;
  const sourceY = (bgPosY / (100 - visibleHeightPercent)) * maxSourceY || 0;

  return {
    sourceX,
    sourceY,
    sourceWidth,
    sourceHeight,
    zoomLevel,
  };
}

// Edge threshold in pixels to determine when the time indicator is near the edge
// of the container and should trigger auto-scrolling behavior.
const EDGE_THRESHOLD = 100;
// Visibility buffer in pixels to determine when the time indicator is considered
// fully visible within the container.
const VISIBILITY_BUFFER = 50;

/**
 * calculateNextUserControlled determines whether the user is currently controlling
 * the timeline's scroll position based on the position of the time indicator,
 * the container width, and whether the user is actively interacting with the timeline.
 *
 * It returns a boolean indicating if the user is in control of the scroll position.
 *
 * @param relativePosition - The current position of the time indicator relative to the container.
 * @param containerWidth - The width of the timeline container.
 * @param isInteracting - Whether the user is currently interacting with the timeline (e.g., dragging).
 * @param currentUserControlled - The current state of user control over the timeline.
 * @returns boolean - True if the user is controlling the timeline, false otherwise.
 */
export function calculateNextUserControlled(
  relativePosition: number,
  containerWidth: number,
  isInteracting: boolean,
  currentUserControlled: boolean
) {
  if (isInteracting) {
    return currentUserControlled;
  }

  const isIndicatorFullyVisible =
    relativePosition >= VISIBILITY_BUFFER &&
    relativePosition <= containerWidth - VISIBILITY_BUFFER;

  const isIndicatorPartiallyVisible =
    relativePosition >= -EDGE_THRESHOLD &&
    relativePosition <= containerWidth + EDGE_THRESHOLD;

  const isApproachingRightEdge =
    relativePosition > containerWidth - EDGE_THRESHOLD &&
    relativePosition <= containerWidth;

  const isApproachingLeftEdge =
    relativePosition < EDGE_THRESHOLD && relativePosition >= 0;

  // User manually scrolled away from the progress line
  if (!isIndicatorPartiallyVisible) {
    return true;
  }

  // Progress line is fully visible, user control can be disabled
  if (isIndicatorFullyVisible) {
    return false;
  }

  // If user was controlling and line is approaching edge, give back control to auto-scroll
  if (
    currentUserControlled &&
    (isApproachingRightEdge || isApproachingLeftEdge)
  ) {
    return false;
  }

  return currentUserControlled;
}

/**
 * shouldAutoScroll determines whether the timeline should auto-scroll
 * based on the position of the time indicator, the container width,
 * and whether the user is interacting with the timeline or has manual control.
 *
 * @param relativePosition - The current position of the time indicator relative to the container.
 * @param containerWidth - The width of the timeline container.
 * @param isInteracting - Whether the user is currently interacting with the timeline (e.g., dragging).
 * @param isUserControlled - Whether the user has manual control over the timeline's scroll position.
 */
export function shouldAutoScroll(
  relativePosition: number,
  containerWidth: number,
  isInteracting: boolean,
  isUserControlled: boolean
) {
  if (isInteracting || isUserControlled) {
    return false;
  }

  const shouldJumpForward = relativePosition > containerWidth - EDGE_THRESHOLD;
  const shouldJumpBackward = relativePosition < 0;

  return shouldJumpForward || shouldJumpBackward;
}

/**
 * calculateAutoScrollOffset computes the new offset needed to auto-scroll the timeline
 * to keep the time indicator within the visible area of the container.
 * It takes into account the current position of the time indicator,
 * the container width, the total timeline width, and whether to force the scroll.
 *
 * @param timePosition - The current position of the time indicator in pixels.
 * @param relativePosition - The position of the time indicator relative to the container.
 * @param containerWidth - The width of the timeline container in pixels.
 * @param timelineWidth - The total width of the timeline in pixels.
 * @param force - Whether to force the scroll regardless of the current position.
 *
 * @returns number - The new offset to apply to the timeline for auto-scrolling.
 */
export function calculateAutoScrollOffset(
  timePosition: number,
  relativePosition: number,
  containerWidth: number,
  timelineWidth: number,
  force?: boolean
) {
  const totalWidth = timelineWidth + LEFT_PADDING;
  const maxOffset = 0;
  const minOffset = Math.min(0, containerWidth - totalWidth);

  if (force) {
    const targetRelativePosition = LEFT_PADDING + VISIBILITY_BUFFER;
    const newOffset = targetRelativePosition - timePosition;

    return Math.max(minOffset, Math.min(maxOffset, newOffset));
  }

  if (relativePosition > containerWidth - EDGE_THRESHOLD) {
    const targetRelativePosition = LEFT_PADDING + VISIBILITY_BUFFER;
    const newOffset = targetRelativePosition - timePosition;

    if (newOffset < minOffset) {
      return minOffset;
    }

    return Math.max(minOffset, Math.min(maxOffset, newOffset));
  }

  if (relativePosition < 0) {
    const targetRelativePosition = containerWidth - VISIBILITY_BUFFER;
    const newOffset = targetRelativePosition - timePosition;

    return Math.max(minOffset, Math.min(maxOffset, newOffset));
  }

  return 0;
}
