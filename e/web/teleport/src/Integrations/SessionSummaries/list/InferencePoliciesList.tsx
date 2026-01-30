import { useCallback, useMemo } from 'react';

import Flex from 'design/Flex';
import { ArrowRight } from 'design/Icon';
import Text, { H3 } from 'design/Text';

import {
  resourceTypeIcon,
  resourceTypeToLabel,
} from 'e-teleport/Integrations/SessionSummaries/helpers';
import {
  ItemLink,
  PaginatedList,
} from 'e-teleport/Integrations/SessionSummaries/list/PaginatedList';
import {
  OverlayEntity,
  useSessionSummariesManagement,
} from 'e-teleport/Integrations/SessionSummaries/SessionSummariesManagement';
import type {
  InferencePolicy,
  ListInferencePoliciesVariables,
} from 'e-teleport/services/inference';
import { useInfiniteListInferencePolicies } from 'e-teleport/services/inference/hooks';
import useStickyClusterId from 'teleport/useStickyClusterId';

import { type ResourceKind } from '../schema/types';

export function InferencePoliciesList() {
  const { clusterId } = useStickyClusterId();

  const queryVariables: ListInferencePoliciesVariables = useMemo(
    () => ({
      clusterId,
      limit: 10,
    }),
    [clusterId]
  );

  const query = useInfiniteListInferencePolicies(queryVariables, {
    getNextPageParam: lastPage =>
      // backend returns an empty string, but we need to give TanStack undefined to indicate no more pages
      lastPage.nextKey?.length > 0 ? lastPage.nextKey : undefined,
    initialPageParam: '',
    staleTime: Infinity,
  });

  const { createEditOverlayLink } = useSessionSummariesManagement();

  const rowRenderer = useCallback(
    (item: InferencePolicy) => {
      const to = createEditOverlayLink(OverlayEntity.Policy, item.name);

      return (
        <ItemLink key={item.name} to={to}>
          <Flex alignItems="center" gap={2} flex={1}>
            <H3>{item.name}</H3>

            <ArrowRight size="small" color="text.muted" />

            <H3 color="text.slightlyMuted">{item.model}</H3>
          </Flex>

          <PolicySessionKinds kinds={item.kinds as ResourceKind[]} />
        </ItemLink>
      );
    },
    [createEditOverlayLink]
  );

  return <PaginatedList query={query} rowRenderer={rowRenderer} />;
}

function PolicySessionKinds({ kinds }: { kinds: ResourceKind[] }) {
  return (
    <Flex flexWrap="wrap" gap={2} color="text.slightlyMuted">
      {kinds.map(kind => (
        <Flex
          alignItems="center"
          border={1}
          borderColor="interactive.tonal.neutral.1"
          borderRadius="8px"
          key={kind}
          px={1}
          py={0.5}
          gap={1}
        >
          {resourceTypeIcon(kind)}

          <Text fontSize="sm">{resourceTypeToLabel(kind)}</Text>
        </Flex>
      ))}
    </Flex>
  );
}
