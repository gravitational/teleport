import { useMemo } from 'react';
import type { FallbackProps } from 'react-error-boundary';
import styled from 'styled-components';

import Flex from 'design/Flex';
import { ErrorSuspenseWrapper } from 'shared/components/ErrorSuspenseWrapper/ErrorSuspenseWrapper';

import cfg from 'e-teleport/config';
import { RECORDING_TYPES_WITH_SUMMARIES } from 'e-teleport/services/recordings/recordings';
import type { RecordingType } from 'teleport/services/recordings';
import { ViewSessionRecordingRoute } from 'teleport/SessionRecordings/view/ViewSessionRecordingRoute';

import {
  SessionSummary,
  SessionSummaryError,
  SessionSummaryLoading,
} from '../list/ViewSummary';

export function ViewSessionRecordingRouteE() {
  const summarySlot = useMemo(() => {
    if (!cfg.oss.sessionSummarizerEnabled) {
      return;
    }

    return (sessionId: string, type: RecordingType) =>
      RECORDING_TYPES_WITH_SUMMARIES.includes(type) ? (
        <ErrorSuspenseWrapper
          errorComponent={SummaryErrorWrapper}
          loadingComponent={SummaryLoadingWrapper}
        >
          <SessionSummary sessionId={sessionId} />
        </ErrorSuspenseWrapper>
      ) : null;
  }, []);

  return <ViewSessionRecordingRoute summarySlot={summarySlot} />;
}

const SessionSummaryContainer = styled(Flex)`
  flex-direction: column;
  justify-content: center;
  width: 100%;
  gap: ${p => p.theme.space[2]}px;
`;

function SummaryErrorWrapper({ error, resetErrorBoundary }: FallbackProps) {
  return (
    <SessionSummaryContainer>
      <SessionSummaryError
        error={error}
        resetErrorBoundary={resetErrorBoundary}
      />
    </SessionSummaryContainer>
  );
}

function SummaryLoadingWrapper() {
  return (
    <SessionSummaryContainer alignItems="center">
      <SessionSummaryLoading />
    </SessionSummaryContainer>
  );
}
