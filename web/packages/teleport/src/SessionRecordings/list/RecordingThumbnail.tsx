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

import { useMemo } from 'react';

import Box from 'design/Box';

import { useSuspenseGetRecordingThumbnail } from 'teleport/services/recordings/hooks';
import { useThumbnailSvg } from 'teleport/SessionRecordings/svg';

interface RecordingThumbnailProps {
  clusterId: string;
  sessionId: string;
  styles: string;
}

// zoomLevel determines how far the thumbnail is zoomed in.
// A higher zoom level means the thumbnail is more zoomed in, showing a smaller area of the recording.
const zoomLevel = 2;

export function RecordingThumbnail({
  clusterId,
  sessionId,
  styles,
}: RecordingThumbnailProps) {
  const { data } = useSuspenseGetRecordingThumbnail(
    {
      clusterId,
      sessionId,
    },
    {
      staleTime: 1000 * 60 * 5, // 5 minutes
      gcTime: 1000 * 60 * 5, // 5 minutes
    }
  );

  // calculate the background position based on the cursor position and zoom level
  const { bgPosX, bgPosY } = useMemo(() => {
    let bgPosX: number;
    let bgPosY: number;

    if (data.cursorVisible) {
      const cursorPercentX = (data.cursorX / data.cols) * 100;
      const cursorPercentY = (data.cursorY / data.rows) * 100;

      const viewportStartX = cursorPercentX - 50 / zoomLevel;
      const viewportStartY = cursorPercentY - 50 / zoomLevel;

      const viewportSize = 100 / zoomLevel;

      bgPosX = (viewportStartX / (100 - viewportSize)) * 100;
      bgPosY = (viewportStartY / (100 - viewportSize)) * 100;

      bgPosX = Math.max(0, Math.min(100, bgPosX));
      bgPosY = Math.max(0, Math.min(100, bgPosY));
    } else {
      bgPosX = 50;
      bgPosY = 50;
    }

    return { bgPosX, bgPosY };
  }, [data.cols, data.cursorX, data.cursorY, data.rows, data.cursorVisible]);

  const dataUri = useThumbnailSvg(data.svg, styles);

  return (
    <Box
      data-testid="recording-thumbnail"
      background={`url("${dataUri}")`}
      backgroundPosition={`${bgPosX}% ${bgPosY}%`}
      backgroundSize={`${zoomLevel * 100}%`}
      height="100%"
      width="100%"
    />
  );
}
