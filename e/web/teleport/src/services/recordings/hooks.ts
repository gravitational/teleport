import { fetchRecordingSummary } from 'e-teleport/services/recordings/recordings';
import { createQueryHook } from 'teleport/services/queryHelpers';

export const { useSuspenseQuery: useSuspenseGetRecordingSummary } =
  createQueryHook(['recording', 'summary'], fetchRecordingSummary);
