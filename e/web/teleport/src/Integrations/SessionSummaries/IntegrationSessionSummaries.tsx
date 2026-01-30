import Text, { H2 } from 'design/Text';

import { IntegrationSessionSummariesHeader } from 'e-teleport/Integrations/SessionSummaries/Header';
import { InferenceModelsList } from 'e-teleport/Integrations/SessionSummaries/list/InferenceModelsList';
import { InferencePoliciesList } from 'e-teleport/Integrations/SessionSummaries/list/InferencePoliciesList';
import { InferenceSecretsList } from 'e-teleport/Integrations/SessionSummaries/list/InferenceSecretsList';
import { SessionSummariesManagementProvider } from 'e-teleport/Integrations/SessionSummaries/SessionSummariesManagement';
import { FeatureBox } from 'teleport/components/Layout';

export function IntegrationSessionSummaries() {
  return (
    <SessionSummariesManagementProvider>
      <IntegrationSessionSummariesHeader />

      <FeatureBox maxWidth={1440} mx="auto" gap={2} py={4} unsetHeight>
        <H2>Inference Policies</H2>

        <Text color="text.slightlyMuted">
          Inference policies allow you to define rules for what kinds of
          sessions should have summaries generated, as well as the model to be
          used for each session type.
        </Text>

        <InferencePoliciesList />

        <H2 mt={4}>Inference Models</H2>

        <Text color="text.slightlyMuted">
          Inference models are the large language models (LLMs) that are used to
          generate session summaries. You can configure different models based
          on your requirements.
        </Text>

        <InferenceModelsList />

        <H2 mt={4}>Inference Secrets</H2>

        <Text color="text.slightlyMuted">
          For OpenAI and OpenAI-compatible models, you may need to provide an
          API key as a secret.
        </Text>

        <InferenceSecretsList />
      </FeatureBox>
    </SessionSummariesManagementProvider>
  );
}
