import { fetchRecordingSummary } from 'e-teleport/services/recordings/recordings';
import { createQueryHook } from 'teleport/services/queryHelpers';

export const {
  useQuery: useGetRecordingSummary,
  useSuspenseQuery: useSuspenseGetRecordingSummary,
} = createQueryHook(['recording', 'summary'], fetchRecordingSummary);
