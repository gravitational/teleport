import { Suspense, useEffect } from 'react';
import { ErrorBoundary } from 'react-error-boundary';

import { Danger } from 'design/Alert';
import Box from 'design/Box';
import Flex from 'design/Flex';
import { Indicator } from 'design/Indicator';
import { ErrorSuspenseWrapper } from 'shared/components/ErrorSuspenseWrapper/ErrorSuspenseWrapper';

import { useLocation, useParams } from 'teleport/components/Router';
import { UrlPlayerParams } from 'teleport/config';
import { getUrlParameter } from 'teleport/services/history';
import { RecordingType } from 'teleport/services/recordings';
import { useSuspenseGetRecordingDuration } from 'teleport/services/recordings/hooks';

import { RecordingPlayer } from './RecordingPlayer';
import {
  ViewTerminalRecording,
  type SummarySlot,
} from './ViewTerminalRecording';

const validRecordingTypes = ['ssh', 'k8s', 'desktop', 'database'];

interface ViewSessionRecordingRouteProps {
  summarySlot?: SummarySlot;
}

export function ViewSessionRecordingRoute({
  summarySlot,
}: ViewSessionRecordingRouteProps) {
  const { sid, clusterId } = useParams<UrlPlayerParams>();
  const { search } = useLocation();

  const recordingType = getUrlParameter(
    'recordingType',
    search
  ) as RecordingType;

  useEffect(() => {
    document.title = `Play ${sid} • ${clusterId}`;
  }, [sid, clusterId]);

  if (recordingType === 'ssh') {
    // If the recording type is SSH, try to load the session metadata (ViewTerminalRecording)
    // and render the SSH player with the session metadata/summary.
    // If that errors (such as during a proxy upgrade), we fall back to the
    // RecordingPlayerWrapper which will fetch the session duration and render the player.
    // This is to ensure that the player can still be rendered even if the session metadata
    // cannot be fetched, allowing users to still view the recording.

    return (
      <Suspense fallback={<RecordingPlayerLoading />}>
        <ErrorBoundary
          fallback={
            <ErrorBoundary fallback={<RecordingPlayerError />}>
              <RecordingPlayerWrapper clusterId={clusterId} sessionId={sid} />
            </ErrorBoundary>
          }
        >
          <ViewTerminalRecording
            clusterId={clusterId}
            sessionId={sid}
            summarySlot={summarySlot}
          />
        </ErrorBoundary>
      </Suspense>
    );
  }

  const validRecordingType = validRecordingTypes.includes(recordingType);
  const durationMs = Number(getUrlParameter('durationMs', search));
  const validDuration = Number.isInteger(durationMs) && durationMs > 0;

  const shouldFetchSessionDuration = !validRecordingType || !validDuration;

  if (shouldFetchSessionDuration) {
    return (
      <ErrorSuspenseWrapper
        errorComponent={RecordingPlayerError}
        loadingComponent={RecordingPlayerLoading}
      >
        <RecordingPlayerWrapper clusterId={clusterId} sessionId={sid} />
      </ErrorSuspenseWrapper>
    );
  }

  return (
    <RecordingPlayer
      clusterId={clusterId}
      sessionId={sid}
      durationMs={durationMs}
      recordingType={recordingType}
    />
  );
}

function RecordingPlayerLoading() {
  return (
    <Flex width="100%" height="100%" flexDirection="column">
      <Box textAlign="center" mx={10} mt={5}>
        <Indicator />
      </Box>
    </Flex>
  );
}

function RecordingPlayerError() {
  return (
    <Flex width="100%" height="100%" flexDirection="column">
      <Box textAlign="center" mx={10} mt={5}>
        <Danger mb={0}>
          Unable to determine the length of this session. The session recording
          may be incomplete or corrupted.
        </Danger>
      </Box>
    </Flex>
  );
}

interface ViewRecordingProps {
  clusterId: string;
  sessionId: string;
}

function RecordingPlayerWrapper({ clusterId, sessionId }: ViewRecordingProps) {
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
