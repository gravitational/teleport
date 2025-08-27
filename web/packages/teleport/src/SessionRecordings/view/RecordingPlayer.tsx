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

import { type RefObject } from 'react';
import styled from 'styled-components';

import type { RecordingType } from 'teleport/services/recordings';
import { DesktopPlayer } from 'teleport/SessionRecordings/view/DesktopPlayer';
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
  ref?: RefObject<PlayerHandle>;
}

const Container = styled.div`
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
`;

export function RecordingPlayer({
  clusterId,
  durationMs,
  onTimeChange,
  onToggleSidebar,
  recordingType,
  sessionId,
  onToggleTimeline,
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
      <SshPlayer
        ref={ref}
        onTimeChange={onTimeChange}
        onToggleSidebar={onToggleSidebar}
        sid={sessionId}
        clusterId={clusterId}
        durationMs={durationMs}
        onToggleTimeline={onToggleTimeline}
      />
    </Container>
  );
}
