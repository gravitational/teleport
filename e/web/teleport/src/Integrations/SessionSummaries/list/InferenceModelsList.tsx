import { useCallback, useMemo, type ComponentType } from 'react';
import styled from 'styled-components';

import Flex from 'design/Flex';
import { BedrockLogo, OpenAIBlossom, TeleportLogo } from 'design/Icon';
import { type IconProps } from 'design/Icon/Icon';
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
  InferenceModel,
  ListInferenceModelsVariables,
} from 'e-teleport/services/inference';
import { useInfiniteListInferenceModels } from 'e-teleport/services/inference/hooks';
import useStickyClusterId from 'teleport/useStickyClusterId';

interface ModelInfoProps {
  model: InferenceModel;
}

export function InferenceModelsList() {
  const { clusterId } = useStickyClusterId();

  const queryVariables: ListInferenceModelsVariables = useMemo(
    () => ({
      clusterId,
      limit: 10,
    }),
    [clusterId]
  );

  const query = useInfiniteListInferenceModels(queryVariables, {
    getNextPageParam: lastPage =>
      // backend returns an empty string, but we need to give TanStack undefined to indicate no more pages
      lastPage.nextKey?.length > 0 ? lastPage.nextKey : undefined,
    initialPageParam: '',
    staleTime: Infinity,
  });

  const { createEditOverlayLink } = useSessionSummariesManagement();

  const rowRenderer = useCallback(
    (item: InferenceModel) => {
      const to = createEditOverlayLink(OverlayEntity.Model, item.name);

      return (
        <ItemLink key={item.name} to={to}>
          <Flex alignItems="center" justifyContent="space-between" width="100%">
            <StyledH3>{item.name}</StyledH3>

            <ModelInfo model={item} />
          </Flex>
        </ItemLink>
      );
    },
    [createEditOverlayLink]
  );

  return <PaginatedList query={query} rowRenderer={rowRenderer} />;
}

function getModelDetails(
  model: InferenceModel
): { icon: ComponentType<IconProps>; label: string } | null {
  if (model.openai) {
    return {
      icon: OpenAIBlossom,
      label: model.openai.modelId,
    };
  }

  if (model.bedrock) {
    if (model.bedrock.modelId === 'teleport-cloud-default') {
      return {
        icon: TeleportLogo,
        label: 'Teleport Cloud AI',
      };
    }

    return {
      icon: BedrockLogo,
      label: model.bedrock.modelId,
    };
  }

  return null;
}

function ModelInfo({ model }: ModelInfoProps) {
  const details = getModelDetails(model);

  if (!details) {
    return null;
  }

  return (
    <Flex
      alignItems="center"
      gap={2}
      maxWidth="400px"
      overflow="hidden"
      color="text.slightlyMuted"
    >
      <details.icon size="small" />

      <TruncateText>{details.label}</TruncateText>
    </Flex>
  );
}

const TruncateText = styled.div`
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
`;

const StyledH3 = styled(H3)`
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
`;
