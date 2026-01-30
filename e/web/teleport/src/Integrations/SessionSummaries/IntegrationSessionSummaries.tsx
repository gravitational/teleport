import Text, { H2 } from 'design/Text';

import { IntegrationSessionSummariesHeader } from 'e-teleport/Integrations/SessionSummaries/Header';
import { InferenceModelsList } from 'e-teleport/Integrations/SessionSummaries/list/InferenceModelsList';
import { SessionSummariesManagementProvider } from 'e-teleport/Integrations/SessionSummaries/SessionSummariesManagement';
import { FeatureBox } from 'teleport/components/Layout';

export function IntegrationSessionSummaries() {
  return (
    <SessionSummariesManagementProvider>
      <IntegrationSessionSummariesHeader />

      <FeatureBox maxWidth={1440} mx="auto" gap={2} py={4} unsetHeight>
        <H2>Inference Models</H2>

        <Text color="text.slightlyMuted">
          Inference models are the large language models (LLMs) that are used to
          generate session summaries. You can configure different models based
          on your requirements.
        </Text>

        <InferenceModelsList />
      </FeatureBox>
    </SessionSummariesManagementProvider>
  );
}
