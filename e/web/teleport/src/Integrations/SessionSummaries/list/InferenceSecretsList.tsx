import { useCallback, useMemo } from 'react';

import { H3 } from 'design/Text';

import {
  ItemLink,
  PaginatedList,
} from 'e-teleport/Integrations/SessionSummaries/list/PaginatedList';
import {
  OverlayEntity,
  useSessionSummariesManagement,
} from 'e-teleport/Integrations/SessionSummaries/SessionSummariesManagement';
import type {
  InferenceSecret,
  ListInferenceSecretsVariables,
} from 'e-teleport/services/inference';
import { useInfiniteListInferenceSecrets } from 'e-teleport/services/inference/hooks';
import useStickyClusterId from 'teleport/useStickyClusterId';

export function InferenceSecretsList() {
  const { clusterId } = useStickyClusterId();

  const queryVariables: ListInferenceSecretsVariables = useMemo(
    () => ({
      clusterId,
      limit: 10,
    }),
    [clusterId]
  );

  const query = useInfiniteListInferenceSecrets(queryVariables, {
    getNextPageParam: lastPage =>
      // backend returns an empty string, but we need to give TanStack undefined to indicate no more pages
      lastPage.nextKey?.length > 0 ? lastPage.nextKey : undefined,
    initialPageParam: '',
    staleTime: Infinity,
  });

  const { createEditOverlayLink } = useSessionSummariesManagement();

  const rowRenderer = useCallback(
    (item: InferenceSecret) => {
      const to = createEditOverlayLink(OverlayEntity.Secret, item.name);

      return (
        <ItemLink key={item.name} to={to}>
          <H3>{item.name}</H3>
        </ItemLink>
      );
    },
    [createEditOverlayLink]
  );

  return <PaginatedList query={query} rowRenderer={rowRenderer} />;
}
