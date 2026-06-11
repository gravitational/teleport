import { http, HttpResponse } from 'msw';

import cfg from 'e-teleport/config';
import {
  RecordingSummaryState,
  type SessionRecordingSummaryResponse,
} from 'e-teleport/services/recordings/types';

export function withMockSummary(
  sessionId: string,
  summary: SessionRecordingSummaryResponse
) {
  return http.get(
    cfg.getSessionRecordingSummaryUrl(':clusterId', sessionId),
    () => HttpResponse.json(summary)
  );
}

export function withSuccessSummary(sessionId: string, content: string) {
  return withMockSummary(sessionId, {
    state: RecordingSummaryState.Success,
    content,
    inferenceFinishedAt: '2025-09-01T12:00:00Z',
    inferenceStartedAt: '2025-09-01T11:59:00Z',
    sessionId,
  });
}

export function withPendingSummary(sessionId: string) {
  return withMockSummary(sessionId, {
    state: RecordingSummaryState.Pending,
    inferenceStartedAt: '2025-09-01T11:59:00Z',
    sessionId,
  });
}

export function withSummaryWithGenerationError(sessionId: string) {
  return withMockSummary(sessionId, {
    state: RecordingSummaryState.Error,
    errorMessage: 'Error generating summary',
    sessionId,
  });
}
