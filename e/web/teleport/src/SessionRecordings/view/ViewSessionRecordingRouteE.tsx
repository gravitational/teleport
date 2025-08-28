import { useCallback } from 'react';
import type { FallbackProps } from 'react-error-boundary';
import styled from 'styled-components';

import Flex from 'design/Flex';
import { ErrorSuspenseWrapper } from 'shared/components/ErrorSuspenseWrapper/ErrorSuspenseWrapper';

import cfg from 'e-teleport/config';
import { ViewSessionRecordingRoute } from 'teleport/SessionRecordings/view/ViewSessionRecordingRoute';

import {
  SessionSummary,
  SessionSummaryError,
  SessionSummaryLoading,
} from '../list/ViewSummary';

export function ViewSessionRecordingRouteE() {
  const summarySlot = useCallback(
    (sessionId: string) => (
      <ErrorSuspenseWrapper
        errorComponent={SummaryErrorWrapper}
        loadingComponent={SummaryLoadingWrapper}
      >
        <SessionSummary sessionId={sessionId} />
      </ErrorSuspenseWrapper>
    ),
    []
  );

  return (
    <ViewSessionRecordingRoute
      summarySlot={cfg.oss.sessionSummarizerEnabled ? summarySlot : undefined}
    />
  );
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
