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
}

const zoomLevel = 5;

export function RecordingThumbnail({
  clusterId,
  sessionId,
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

  const { bgPosX, bgPosY } = useMemo(() => {
    const visibleWidthPercent = 100 / zoomLevel;
    const visibleHeightPercent = 100 / zoomLevel;

    const cursorPercentX = (data.cursorX / data.cols) * 100;
    const cursorPercentY = (data.cursorY / data.rows) * 100;

    const bgPosX = Math.max(
      0,
      Math.min(
        100 - visibleWidthPercent,
        cursorPercentX - visibleWidthPercent / 2
      )
    );
    const bgPosY = Math.max(
      0,
      Math.min(
        100 - visibleHeightPercent,
        cursorPercentY - visibleHeightPercent / 2
      )
    );

    return { bgPosX, bgPosY };
  }, [data.cols, data.cursorX, data.cursorY, data.rows]);

  const dataUri = useThumbnailSvg(data.svg);

  return (
    <Box
      data-testid="recording-thumbnail"
      bg={`url("${dataUri}")`}
      backgroundPosition={`${bgPosX}% ${bgPosY}%`}
      backgroundSize="400%"
      height="100%"
      width="100%"
    />
  );
}
