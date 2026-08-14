import { useCallback, type ComponentType } from 'react';
import {
  useController,
  useFormContext,
  useWatch,
  type FieldPathValue,
} from 'react-hook-form';

import Flex, { Stack } from 'design/Flex';
import { Sparkle, TeleportLogo } from 'design/Icon';
import { type IconProps } from 'design/Icon/Icon';
import Text, { H3 } from 'design/Text';

import {
  AccessMethodOptionContainer,
  RadioCircle,
  RadioDot,
} from 'e-teleport/SessionRecordings/setup/fields/common';
import { TELEPORT_CLOUD_MODEL } from 'e-teleport/SessionRecordings/setup/schema/accessMethods';
import type { InferencePolicySchema } from 'e-teleport/SessionRecordings/setup/schema/policy';

interface AvailableAccessMethod {
  description: string;
  Icon: ComponentType<IconProps>;
  text: string;
  isProvidedByCloud: boolean;
}

const accessMethods: AvailableAccessMethod[] = [
  {
    description: 'Use Claude Sonnet 4.5 provided by Teleport Cloud',
    Icon: TeleportLogo,
    text: 'Teleport Cloud',
    isProvidedByCloud: true,
  },
  {
    description:
      'Use a model from OpenAI, an OpenAI-compatible API, or Amazon Bedrock',
    Icon: Sparkle,
    text: 'Use your own model',
    isProvidedByCloud: false,
  },
];

export function FieldCloudModelProvider() {
  const { setFocus, setValue } = useFormContext<InferencePolicySchema>();

  const { field } = useController<
    InferencePolicySchema,
    'providedByTeleportCloud'
  >({
    name: 'providedByTeleportCloud',
  });

  const handleAccessMethodChange = useCallback(
    (
      value: FieldPathValue<InferencePolicySchema, 'providedByTeleportCloud'>
    ) => {
      field.onChange(value);

      if (value) {
        setValue('model', TELEPORT_CLOUD_MODEL, { shouldValidate: true });
      } else {
        setValue('model', '', { shouldValidate: true });

        requestAnimationFrame(() => {
          setFocus('model');
        });
      }
    },
    [field, setFocus, setValue]
  );

  return (
    <Flex alignItems="stretch" width="100%" gap={3}>
      {accessMethods.map(method => (
        <AccessMethodOption
          details={method}
          key={method.text}
          onChange={handleAccessMethodChange}
        />
      ))}
    </Flex>
  );
}

interface AccessMethodOptionProps {
  details: AvailableAccessMethod;
  onChange: (
    value: FieldPathValue<InferencePolicySchema, 'providedByTeleportCloud'>
  ) => void;
}

function AccessMethodOption({ details, onChange }: AccessMethodOptionProps) {
  const value = useWatch<InferencePolicySchema, 'providedByTeleportCloud'>({
    name: 'providedByTeleportCloud',
  });

  const selected = details.isProvidedByCloud === value;

  return (
    <AccessMethodOptionContainer
      onClick={() => {
        onChange(details.isProvidedByCloud);
      }}
      selected={selected}
    >
      <Flex alignItems="center" height="32px" justifyContent="center">
        <RadioCircle selected={selected}>
          {selected && <RadioDot />}
        </RadioCircle>
      </Flex>

      <Stack flexDirection="row" gap={3} width="100%">
        <Flex alignItems="center" height="32px" justifyContent="center">
          <details.Icon size="medium" />
        </Flex>

        <Stack gap={0} flex={1} pr={3}>
          <Flex alignItems="center" height="32px">
            <H3>{details.text}</H3>
          </Flex>

          <Text color="text.slightlyMuted">{details.description}</Text>
        </Stack>
      </Stack>
    </AccessMethodOptionContainer>
  );
}
