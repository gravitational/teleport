export enum RecordingSummaryState {
  Pending = 'SUMMARY_STATE_PENDING',
  Success = 'SUMMARY_STATE_SUCCESS',
  Error = 'SUMMARY_STATE_ERROR',
}

interface BaseRecordingSummary {
  sessionId: string;
}

interface RecordingSummaryPending extends BaseRecordingSummary {
  inferenceStartedAt: string;
  state: RecordingSummaryState.Pending;
}

interface RecordingSummarySuccess extends BaseRecordingSummary {
  state: RecordingSummaryState.Success;
  content: string;
  inferenceStartedAt: string;
  inferenceFinishedAt: string;
}

interface RecordingSummaryError extends BaseRecordingSummary {
  state: RecordingSummaryState.Error;
  errorMessage: string;
}

export type SessionRecordingSummary =
  | RecordingSummaryPending
  | RecordingSummarySuccess
  | RecordingSummaryError;
