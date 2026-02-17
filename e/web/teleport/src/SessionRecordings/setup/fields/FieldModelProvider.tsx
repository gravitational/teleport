import { useCallback, useMemo, type ComponentType } from 'react';
import {
  useController,
  useFormContext,
  useWatch,
  type FieldPathValue,
} from 'react-hook-form';
import styled from 'styled-components';

import Flex, { Stack } from 'design/Flex';
import { BedrockLogo, OpenAIBlossom, TeleportLogo } from 'design/Icon';
import { type IconProps } from 'design/Icon/Icon';
import { LabelContent } from 'design/LabelInput/LabelInput';
import Text, { H3 } from 'design/Text';

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

interface SelectProviderProps {
  isCloud: boolean;
}

export function FieldModelProvider({ isCloud }: SelectProviderProps) {
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
      getAvailableAccessMethods(isCloud).map(method => (
        <AccessMethodOption
          details={method}
          key={method.text}
          onChange={handleAccessMethodChange}
        />
      )),
    [isCloud, handleAccessMethodChange]
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

const AccessMethodOptionContainer = styled.div<{ selected: boolean }>`
  display: flex;
  align-items: flex-start;
  gap: ${p => p.theme.space[3]}px;
  padding: ${p => p.theme.space[2]}px ${p => p.theme.space[3]}px
    ${p => p.theme.space[3]}px;
  border: 1px solid
    ${p =>
      p.selected
        ? p.theme.colors.brand
        : p.theme.colors.interactive.tonal.neutral[0]};
  border-radius: ${p => p.theme.radii[3]}px;
  cursor: pointer;
  flex: 1;

  &:hover {
    background-color: ${p => p.theme.colors.interactive.tonal.neutral[1]};
  }
`;

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

function getAvailableAccessMethods(isCloud: boolean) {
  const methods: AvailableAccessMethod[] = [];

  if (isCloud) {
    methods.push({
      description: 'Use Claude Sonnet 4.5 provided by Teleport Cloud',
      Icon: TeleportLogo,
      text: 'Teleport Cloud',
      values: ['teleport' as const],
    });
  }

  methods.push(
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
    }
  );

  return methods;
}

const RadioCircle = styled.div<{ selected: boolean }>`
  display: flex;
  align-items: center;
  justify-content: center;
  width: 14px;
  height: 14px;
  border: 1px solid
    ${p =>
      p.selected
        ? p.theme.colors.interactive.solid.primary.default
        : p.theme.colors.interactive.tonal.neutral[2]};
  border-radius: 50%;
  background: transparent;
`;

const RadioDot = styled.div`
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: ${p => p.theme.colors.interactive.solid.primary.default};
`;
