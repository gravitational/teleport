import { forwardRef } from 'react';
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
  showTimeline?: () => void;
}

const Container = styled.div`
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
`;

export const RecordingPlayer = forwardRef<PlayerHandle, RecordingPlayerProps>(
  function RecordingPlayer(
    {
      clusterId,
      durationMs,
      onTimeChange,
      recordingType,
      sessionId,
      showTimeline,
    },
    ref
  ) {
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
          sid={sessionId}
          clusterId={clusterId}
          durationMs={durationMs}
          showTimeline={showTimeline}
        />
      </Container>
    );
  }
);
