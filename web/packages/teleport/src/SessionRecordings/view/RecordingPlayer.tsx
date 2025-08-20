import type { RecordingType } from 'teleport/services/recordings';

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
  return <div>player</div>;
}
