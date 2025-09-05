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

import { Suspense, type RefObject } from 'react';
import { ErrorBoundary } from 'react-error-boundary';
import styled from 'styled-components';

import Flex from 'design/Flex';
import Indicator from 'design/Indicator';

import type { RecordingType } from 'teleport/services/recordings';
import { DesktopPlayer } from 'teleport/SessionRecordings/view/DesktopPlayer';
import { TtyRecordingPlayer } from 'teleport/SessionRecordings/view/player/tty/TtyRecordingPlayer';
import SshPlayer, {
  type PlayerHandle,
} from 'teleport/SessionRecordings/view/SshPlayer';

interface RecordingPlayerProps {
  clusterId: string;
  durationMs: number;
  onTimeChange?: (time: number) => void;
  recordingType: RecordingType;
  sessionId: string;
  onToggleSidebar?: () => void;
  onToggleTimeline?: () => void;
  onToggleFullscreen?: () => void;
  fullscreen?: boolean;
  initialCols?: number;
  initialRows?: number;
  ref?: RefObject<PlayerHandle>;
}

const Container = styled.div`
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
`;

/**
 * RecordingPlayer is a wrapper component that chooses between different types of recording players
 * (e.g., DesktopPlayer, TtyRecordingPlayer) based on the recording type.
 *
 * It will fall back to the legacy SshPlayer if TtyRecordingPlayer fails to load (e.g. the WebSocket
 * fails to open due to a proxy mismatch).
 *
 * TODO(ryan): DELETE in v20
 */
export function RecordingPlayer({
  clusterId,
  sessionId,
  durationMs,
  fullscreen,
  onTimeChange,
  onToggleFullscreen,
  onToggleSidebar,
  onToggleTimeline,
  recordingType,
  initialCols,
  initialRows,
  ref,
}: RecordingPlayerProps) {
  if (recordingType === 'desktop') {
    return (
      <Container>
        <DesktopPlayer
          sid={sessionId}
          clusterId={clusterId}
          durationMs={durationMs}
        />
      </Container>
    );
  }

  return (
    <Container>
      <ErrorBoundary
        fallback={
          <SshPlayer
            ref={ref}
            onTimeChange={onTimeChange}
            onToggleSidebar={onToggleSidebar}
            sid={sessionId}
            clusterId={clusterId}
            durationMs={durationMs}
            onToggleTimeline={onToggleTimeline}
          />
        }
      >
        <Suspense fallback={<RecordingPlayerLoading />}>
          <TtyRecordingPlayer
            clusterId={clusterId}
            sessionId={sessionId}
            duration={durationMs}
            fullscreen={fullscreen}
            onToggleFullscreen={onToggleFullscreen}
            onTimeChange={onTimeChange}
            onToggleSidebar={onToggleSidebar}
            onToggleTimeline={onToggleTimeline}
            initialCols={initialCols}
            initialRows={initialRows}
            ref={ref}
          />
        </Suspense>
      </ErrorBoundary>
    </Container>
  );
}

function RecordingPlayerLoading() {
  return (
    <Flex alignItems="center" justifyContent="center" height="100%">
      <Indicator />
    </Flex>
  );
}
