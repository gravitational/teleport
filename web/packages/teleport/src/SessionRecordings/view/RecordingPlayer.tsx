import type { RecordingType } from 'teleport/services/recordings';
import { DesktopPlayer } from 'teleport/SessionRecordings/view/DesktopPlayer';
import SshPlayer from 'teleport/SessionRecordings/view/SshPlayer';

interface RecordingPlayerProps {
  clusterId: string;
  durationMs: number;
  recordingType: RecordingType;
  sessionId: string;
}

export function RecordingPlayer({
  clusterId,
  durationMs,
  recordingType,
  sessionId,
}: RecordingPlayerProps) {
  if (recordingType === 'desktop') {
    return (
      <DesktopPlayer
        sid={sessionId}
        clusterId={clusterId}
        durationMs={durationMs}
      />
    );
  }

  return (
    <SshPlayer sid={sessionId} clusterId={clusterId} durationMs={durationMs} />
  );
}
