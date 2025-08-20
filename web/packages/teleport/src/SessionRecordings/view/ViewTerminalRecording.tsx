import { useGetRecordingMetadata } from 'teleport/services/recordings/hooks';

interface ViewTerminalRecordingProps {
  clusterId: string;
  sessionId: string;
}

export function ViewTerminalRecording({
  clusterId,
  sessionId,
}: ViewTerminalRecordingProps) {
  const { data, isError, isPending } = useGetRecordingMetadata({
    clusterId,
    sessionId,
  });

  console.log('Recording metadata:', data);

  if (isError) {
    return <div>Error loading recording metadata.</div>;
  }

  if (isPending) {
    return <div>Loading...</div>;
  }

  return <div>hello</div>;
}
