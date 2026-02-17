import { useMemo } from 'react';
import { Link } from 'react-router-dom';

import { ButtonSecondary } from 'design/Button';
import Flex from 'design/Flex';
import Text, { H2 } from 'design/Text';

import { IntegrationSessionSummariesHeader } from 'e-teleport/SessionRecordings/setup/Header';
import { InferenceModelsList } from 'e-teleport/SessionRecordings/setup/list/InferenceModelsList';
import { InferencePoliciesList } from 'e-teleport/SessionRecordings/setup/list/InferencePoliciesList';
import { InferenceSecretsList } from 'e-teleport/SessionRecordings/setup/list/InferenceSecretsList';
import {
  OverlayEntity,
  SessionSummariesManagementProvider,
  useSessionSummariesManagement,
} from 'e-teleport/SessionRecordings/setup/SessionSummariesManagement';
import { FeatureBox } from 'teleport/components/Layout';

export function SessionSummaries() {
  return (
    <SessionSummariesManagementProvider>
      <IntegrationSessionSummariesHeader />

      <FeatureBox maxWidth={1440} mx="auto" gap={2} py={4} unsetHeight>
        <SectionHeader entity={OverlayEntity.Policy} />

        <Text color="text.slightlyMuted">
          Inference policies allow you to define rules for what kinds of
          sessions should have summaries generated, as well as the model to be
          used for each session type.
        </Text>

        <InferencePoliciesList />

        <SectionHeader entity={OverlayEntity.Model} mt={4} />

        <Text color="text.slightlyMuted">
          Inference models are the large language models (LLMs) that are used to
          generate session summaries. You can configure different models based
          on your requirements.
        </Text>

        <InferenceModelsList />

        <SectionHeader entity={OverlayEntity.Secret} mt={4} />

        <Text color="text.slightlyMuted">
          For OpenAI and OpenAI-compatible models, you may need to provide an
          API key as a secret.
        </Text>

        <InferenceSecretsList />
      </FeatureBox>
    </SessionSummariesManagementProvider>
  );
}

interface SectionHeaderProps {
  entity: OverlayEntity;
  mt?: number;
}

function SectionHeader({ entity, mt }: SectionHeaderProps) {
  const { singular, plural } = entityToWords(entity);
  const { createNewOverlayLink } = useSessionSummariesManagement();

  const link = useMemo(
    () => createNewOverlayLink(entity),
    [createNewOverlayLink, entity]
  );

  return (
    <Flex alignItems="center" justifyContent="space-between" mt={mt}>
      <H2>{plural}</H2>

      <ButtonSecondary as={Link} to={link} px={3}>
        Add {singular}
      </ButtonSecondary>
    </Flex>
  );
}

function entityToWords(entity: OverlayEntity) {
  switch (entity) {
    case OverlayEntity.Policy:
      return { singular: 'Inference Policy', plural: 'Inference Policies' };
    case OverlayEntity.Model:
      return { singular: 'Inference Model', plural: 'Inference Models' };
    case OverlayEntity.Secret:
      return { singular: 'Inference Secret', plural: 'Inference Secrets' };
    default:
      return { singular: '', plural: '' };
  }
}
