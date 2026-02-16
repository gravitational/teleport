import { useCallback, useEffect, useRef } from 'react';
import { useFormContext, useWatch } from 'react-hook-form';
import styled from 'styled-components';

import Box from 'design/Box';
import Flex, { Stack } from 'design/Flex';
import { BedrockLogo } from 'design/Icon';
import {
  TabBorder,
  TabContainer,
  TabsContainer,
  useSlidingBottomBorderTabs,
} from 'design/Tabs';
import Text, { H3 } from 'design/Text';

import cfg from 'e-teleport/config';
import { FieldInput } from 'e-teleport/Integrations/SessionSummaries/fields/FieldInput';
import { FieldModelSelect } from 'e-teleport/Integrations/SessionSummaries/fields/FieldModelSelect';
import { FieldRegion } from 'e-teleport/Integrations/SessionSummaries/fields/FieldRegion';
import { FieldTeleportAWSIntegration } from 'e-teleport/Integrations/SessionSummaries/fields/FieldTeleportAWSIntegration';
import type { InferenceModelSchema } from 'e-teleport/Integrations/SessionSummaries/schema/model';

enum Tab {
  TeleportIntegration = 'integration',
  InferenceProfile = 'inference_profile',
  Direct = 'direct',
}

const SmallTab = styled(TabContainer)`
  font-size: 14px;
  padding: 0 ${props => props.theme.space[2]}px;
`;

interface BedrockConfigurationFormProps {
  isCloud: boolean;
}

export function BedrockConfigurationForm({
  isCloud,
}: BedrockConfigurationFormProps) {
  const form = useFormContext<InferenceModelSchema>();

  const activeTab = useWatch<InferenceModelSchema, 'bedrockMode'>({
    name: 'bedrockMode',
  });

  const { borderRef, parentRef } = useSlidingBottomBorderTabs({ activeTab });

  useEffect(() => {
    switch (activeTab) {
      case Tab.TeleportIntegration:
        requestAnimationFrame(() => form.setFocus('region'));

        break;

      case Tab.Direct:
        requestAnimationFrame(() => form.setFocus('region'));

        break;

      case Tab.InferenceProfile:
        requestAnimationFrame(() => form.setFocus('model'));

        break;
    }
  }, [activeTab, form]);

  const createTabHandler = useCallback(
    (tab: Tab) => () => {
      form.setValue('bedrockMode', tab, {
        shouldValidate: true,
      });

      if (tab === Tab.Direct) {
        requestAnimationFrame(() => form.setFocus('model'));
      }

      if (tab === Tab.InferenceProfile) {
        requestAnimationFrame(() => form.setFocus('inferenceProfile'));
      }
    },
    [form]
  );

  return (
    <Box
      border="1px solid"
      borderColor="interactive.tonal.neutral.1"
      px={3}
      py={3}
      borderRadius={3}
    >
      <Flex alignItems="center" mb={3} gap={2}>
        <BedrockLogo size="medium" />

        <H3>Bedrock Model Configuration</H3>
      </Flex>

      {!isCloud && (
        <TabsContainer ref={parentRef} mb={3} withBottomBorder={true}>
          <SmallTab
            data-tab-id={Tab.TeleportIntegration}
            selected={activeTab === Tab.TeleportIntegration}
            onClick={createTabHandler(Tab.TeleportIntegration)}
          >
            Use Teleport AWS integration
          </SmallTab>
          <SmallTab
            data-tab-id={Tab.Direct}
            selected={activeTab === Tab.Direct}
            onClick={createTabHandler(Tab.Direct)}
          >
            Connect directly to a model
          </SmallTab>
          <SmallTab
            data-tab-id={Tab.InferenceProfile}
            selected={activeTab === Tab.InferenceProfile}
            onClick={createTabHandler(Tab.InferenceProfile)}
          >
            Use an inference profile
          </SmallTab>
          <TabBorder ref={borderRef} />
        </TabsContainer>
      )}

      <Stack gap={3}>
        {activeTab === Tab.TeleportIntegration && (
          <>
            <Text color="text.slightlyMuted">
              Connect to AWS through OIDC using a Teleport AWS integration.{' '}
              {!cfg.oss.isCloud && (
                <strong>
                  Your Teleport cluster must be publicly accessible to use this
                  method.
                </strong>
              )}
            </Text>

            <Flex gap={4} width="100%">
              <Box flex="0 0 250px">
                <FieldRegion />
              </Box>

              <Box flex={1}>
                <FieldModelSelect />
              </Box>
            </Flex>

            <Stack gap={1} width="100%">
              <Divider />

              <FieldTeleportAWSIntegration />
            </Stack>
          </>
        )}

        {activeTab === Tab.InferenceProfile && (
          <>
            <Text color="text.slightlyMuted">
              Teleport will use the specified inference profile to connect to
              Bedrock and access the model.
            </Text>

            <Flex gap={4} width="100%">
              <Box flex={1}>
                <FieldInferenceProfileARN />
              </Box>

              <Box flex="0 0 250px">
                <FieldRegion />
              </Box>
            </Flex>
          </>
        )}

        {activeTab === Tab.Direct && (
          <>
            <Text color="text.slightlyMuted">
              To connect directly to a Bedrock model, please ensure you have the
              necessary AWS permissions configured on the Teleport Auth Servers.
            </Text>

            <Flex gap={4} width="100%">
              <Box flex="0 0 250px">
                <FieldRegion />
              </Box>

              <Box flex={1}>
                <FieldModelSelect />
              </Box>
            </Flex>
          </>
        )}
      </Stack>
    </Box>
  );
}

function FieldInferenceProfileARN() {
  const form = useFormContext<InferenceModelSchema>();
  const value = useWatch<InferenceModelSchema, 'inferenceProfile'>({
    name: 'inferenceProfile',
  });

  const lastValue = useRef<string | undefined>(undefined);

  useEffect(() => {
    if (form.getValues('region')) {
      return;
    }

    if (!value || value === lastValue.current) {
      return;
    }

    lastValue.current = value;

    const region = getRegionFromArn(value);

    if (region) {
      form.setValue('region', region, {
        shouldDirty: true,
        shouldTouch: true,
        shouldValidate: true,
      });
    }
  }, [form, value]);

  return (
    <FieldInput
      label="Inference Profile ARN"
      name="inferenceProfile"
      placeholder="arn:aws:bedrock:us-west-2:123456789012:inference-profile/your-profile"
      required={true}
    />
  );
}

function getRegionFromArn(arn: string) {
  const match = /^arn:aws:[^:]+:([^:]+):\d+:/.exec(arn);
  return match ? match[1] : null;
}

const Divider = styled.hr`
  border: none;
  border-top: 1px solid ${p => p.theme.colors.interactive.tonal.neutral[1]};
  margin: ${p => p.theme.space[3]}px 0;
  width: 100%;
`;
