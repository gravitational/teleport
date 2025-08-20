import { useSuspenseGetRecordingDuration } from 'teleport/services/recordings/hooks';
import { RecordingPlayer } from 'teleport/SessionRecordings/view/RecordingPlayer';

interface RecordingPlayerWrapperProps {
  clusterId: string;
  sessionId: string;
}

export function RecordingPlayerWrapper({
  clusterId,
  sessionId,
}: RecordingPlayerWrapperProps) {
  const { data } = useSuspenseGetRecordingDuration({
    clusterId,
    sessionId,
  });

  return (
    <RecordingPlayer
      clusterId={clusterId}
      sessionId={sessionId}
      durationMs={data.durationMs}
      recordingType={data.recordingType}
    />
  );
}
