import type { SessionRecordingThumbnail } from 'teleport/services/recordings';
import { LEFT_PADDING } from 'teleport/SessionRecordings/view/Timeline/constants';

const zoomLevelWithCursor = 4; // Zoom in when cursor is visible
const zoomLevelWithoutCursor = 1.5; // Less zoom when cursor is not visible

// calculateThumbnailViewport calculates the portion of the thumbnail image to display
// based on the cursor position and whether the cursor is visible.
// It returns the source coordinates and dimensions to be used in drawing the image,
// as well as the zoom level applied.
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

const EDGE_THRESHOLD = 100;
const VISIBILITY_BUFFER = 50;

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
