import cfg from 'e-teleport/config';
import { type SessionRecordingSummary } from 'e-teleport/services/recordings/types';
import api from 'teleport/services/api';

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

  return response as SessionRecordingSummary;
}
