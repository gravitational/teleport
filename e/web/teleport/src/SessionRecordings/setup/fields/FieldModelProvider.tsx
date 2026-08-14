import { useCallback, useMemo, type ComponentType } from 'react';
import {
  useController,
  useFormContext,
  useWatch,
  type FieldPathValue,
} from 'react-hook-form';

import Flex, { Stack } from 'design/Flex';
import { BedrockLogo, OpenAIBlossom } from 'design/Icon';
import { type IconProps } from 'design/Icon/Icon';
import { LabelContent } from 'design/LabelInput/LabelInput';
import Text, { H3 } from 'design/Text';

import {
  AccessMethodOptionContainer,
  RadioCircle,
  RadioDot,
} from 'e-teleport/SessionRecordings/setup/fields/common';
import type { InferenceModelSchema } from 'e-teleport/SessionRecordings/setup/schema/model';

interface AccessMethodOptionProps {
  details: AvailableAccessMethod;
  onChange: (
    value: FieldPathValue<InferenceModelSchema, 'accessMethod'>
  ) => void;
}

interface AvailableAccessMethod {
  description: string;
  Icon: ComponentType<IconProps>;
  text: string;
  values: FieldPathValue<InferenceModelSchema, 'accessMethod'>[];
}

const accessMethods = [
  {
    description: 'Use OpenAI or an OpenAI-compatible API',
    Icon: OpenAIBlossom,
    text: 'OpenAI-Compatible API',
    values: ['openai_api' as const, 'openai_compatible' as const],
  },
  {
    description: 'Connect to models through Amazon Bedrock',
    Icon: BedrockLogo,
    text: 'Amazon Bedrock',
    values: ['bedrock' as const],
  },
];

export function FieldModelProvider() {
  const { setFocus, setValue } = useFormContext<InferenceModelSchema>();

  const { field } = useController<InferenceModelSchema, 'accessMethod'>({
    name: 'accessMethod',
  });

  const handleAccessMethodChange = useCallback(
    (value: FieldPathValue<InferenceModelSchema, 'accessMethod'>) => {
      field.onChange(value);

      setValue('model', '');

      if (value === 'bedrock') {
        setValue('bedrockMode', 'integration');
      }

      if (value === 'openai_compatible') {
        requestAnimationFrame(() => {
          setFocus('apiUrl');
        });
      }
    },
    [field, setFocus, setValue]
  );

  const options = useMemo(
    () =>
      accessMethods.map(method => (
        <AccessMethodOption
          details={method}
          key={method.text}
          onChange={handleAccessMethodChange}
        />
      )),
    [handleAccessMethodChange]
  );

  return (
    <Stack>
      <LabelContent required>Choose model provider</LabelContent>

      <Flex alignItems="center" width="100%" gap={3}>
        {options}
      </Flex>
    </Stack>
  );
}

function AccessMethodOption({ details, onChange }: AccessMethodOptionProps) {
  const value = useWatch<InferenceModelSchema, 'accessMethod'>({
    name: 'accessMethod',
  });

  const selected = details.values.includes(value);

  return (
    <AccessMethodOptionContainer
      onClick={() => {
        onChange(details.values[0]);
      }}
      selected={selected}
    >
      <Flex alignItems="center" height="32px" justifyContent="center">
        <RadioCircle selected={selected}>
          {selected && <RadioDot />}
        </RadioCircle>
      </Flex>

      <Stack flexDirection="row" gap={3}>
        <Flex alignItems="center" height="32px" justifyContent="center">
          <details.Icon size="medium" />
        </Flex>

        <Stack gap={0}>
          <Flex alignItems="center" height="32px">
            <H3>{details.text}</H3>
          </Flex>

          <Text color="text.slightlyMuted">{details.description}</Text>
        </Stack>
      </Stack>
    </AccessMethodOptionContainer>
  );
}
