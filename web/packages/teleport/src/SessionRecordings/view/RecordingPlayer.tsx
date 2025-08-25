import styled from 'styled-components';

import type { RecordingType } from 'teleport/services/recordings';
import { DesktopPlayer } from 'teleport/SessionRecordings/view/DesktopPlayer';
import SshPlayer from 'teleport/SessionRecordings/view/SshPlayer';

interface RecordingPlayerProps {
  clusterId: string;
  durationMs: number;
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

export function RecordingPlayer({
  clusterId,
  durationMs,
  recordingType,
  sessionId,
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
        sid={sessionId}
        clusterId={clusterId}
        durationMs={durationMs}
      />
    </Container>
  );
}
