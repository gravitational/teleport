import cfg from 'e-teleport/config';
import { normalizeSessionRecordingSummary } from 'e-teleport/services/recordings/normalize';
import {
  type SessionRecordingSummary,
  type SessionRecordingSummaryResponse,
} from 'e-teleport/services/recordings/types';
import api from 'teleport/services/api';
import type { RecordingType } from 'teleport/services/recordings';

interface FetchRecordingSummaryVariables {
  clusterId: string;
  sessionId: string;
}

export async function fetchRecordingSummary(
  { clusterId, sessionId }: FetchRecordingSummaryVariables,
  signal?: AbortSignal
): Promise<SessionRecordingSummary> {
  const url = cfg.getSessionRecordingSummaryUrl(clusterId, sessionId);
  const response = await api.get(url, signal);

  if (!response) {
    throw new Error('Failed to fetch recording summary');
  }

  return normalizeSessionRecordingSummary(
    response as SessionRecordingSummaryResponse
  );
}

export const RECORDING_TYPES_WITH_SUMMARIES: RecordingType[] = [
  'ssh',
  'k8s',
  'database',
];
