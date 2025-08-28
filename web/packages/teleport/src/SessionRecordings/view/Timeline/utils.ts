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

const zoomLevelWithCursor = 2;
const zoomLevelWithoutCursor = 1;

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
