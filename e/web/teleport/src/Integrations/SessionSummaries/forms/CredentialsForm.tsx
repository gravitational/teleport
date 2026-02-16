import { useCallback, useMemo, type MouseEvent } from 'react';
import { useWatch } from 'react-hook-form';
import { Link } from 'react-router-dom';
import type { SingleValueProps } from 'react-select';
import styled from 'styled-components';

import Box from 'design/Box';
import Flex from 'design/Flex';
import type { Option } from 'shared/components/Select';

import { FieldInput } from 'e-teleport/Integrations/SessionSummaries/fields/FieldInput';
import { FieldSelect } from 'e-teleport/Integrations/SessionSummaries/fields/FieldSelect';
import { BedrockConfigurationForm } from 'e-teleport/Integrations/SessionSummaries/forms/BedrockConfigurationForm';
import type { InferenceModelSchema } from 'e-teleport/Integrations/SessionSummaries/schema/model';
import {
  OverlayEntity,
  useSessionSummariesManagement,
} from 'e-teleport/Integrations/SessionSummaries/SessionSummariesManagement';
import { useListInferenceSecrets } from 'e-teleport/services/inference/hooks';
import useStickyClusterId from 'teleport/useStickyClusterId';

export function CredentialsForm() {
  const accessMethod = useWatch<InferenceModelSchema, 'accessMethod'>({
    name: 'accessMethod',
  });

  if (accessMethod === 'bedrock') {
    return <BedrockConfigurationForm />;
  }

  return <OpenAIConfigurationForm />;
}

function OpenAIConfigurationForm() {
  const { clusterId } = useStickyClusterId();
  const inferenceSecrets = useListInferenceSecrets({ clusterId });

  const { createNewOverlayLink } = useSessionSummariesManagement();

  const options = useMemo(
    () =>
      inferenceSecrets.data?.items?.map(secret => ({
        value: secret.name,
        label: secret.name,
      })) ?? [],
    [inferenceSecrets.data?.items]
  );

  const addInferenceSecretLink = useMemo(
    () => createNewOverlayLink(OverlayEntity.Secret),
    [createNewOverlayLink]
  );

  return (
    <Flex flexDirection="column" gap={4} width="100%">
      <FieldInput
        helperText="Leave blank if using OpenAI"
        label="API URL"
        name="apiUrl"
        placeholder="https://api.your-openai-compatible.com/v1"
      />

      <FieldSelect
        name="secretName"
        components={{
          SingleValue: CustomSingleValue,
        }}
        options={options}
        label="Secret to use for authentication"
        labelButton={
          <AddLink to={addInferenceSecretLink}>
            Add a new inference secret
          </AddLink>
        }
        required={true}
      />

      <FieldInput
        helperText="The name of the model to use from the provider."
        label="Model name"
        name="model"
        placeholder="gpt-5.2"
        required={true}
      />
    </Flex>
  );
}

const SingleValueContainer = styled.div`
  display: flex;
  align-items: center;
  gap: 8px;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  grid-area: 1 / 1 / 2 / 3;
  margin-inline-end: 0.125rem;
  margin-inline-start: 0.125rem;
`;

function CustomSingleValue(props: SingleValueProps<Option>) {
  const secretName = useWatch<{ secretName: string }>({ name: 'secretName' });

  const { openEditOverlay } = useSessionSummariesManagement();

  const handleEditSecret = useCallback(
    (event: MouseEvent) => {
      event.preventDefault();
      event.stopPropagation();

      openEditOverlay(OverlayEntity.Secret, secretName);
    },
    [secretName, openEditOverlay]
  );

  return (
    <SingleValueContainer {...props.innerProps}>
      <Box>{props.data.label}</Box>

      <Spacer />

      <EditSecret
        onClick={handleEditSecret}
        onMouseDown={e => {
          e.preventDefault();
          e.stopPropagation();
        }}
      >
        Edit Secret
      </EditSecret>
    </SingleValueContainer>
  );
}

const EditSecret = styled.div`
  color: ${p => p.theme.colors.brand};
  cursor: pointer;
  font-size: ${p => p.theme.fontSizes[0]};
  line-height: 1;
  position: relative;
  z-index: 100;

  &:hover {
    text-decoration: underline;
  }
`;

const AddLink = styled(Link)`
  color: ${p => p.theme.colors.brand};
  text-decoration: none;

  &:hover {
    text-decoration: underline;
  }
`;

const Spacer = styled.div`
  flex: 1;
`;
