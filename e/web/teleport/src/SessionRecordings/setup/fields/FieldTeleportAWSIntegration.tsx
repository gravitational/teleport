import { useQuery } from '@tanstack/react-query';
import {
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
} from 'react';
import { useFormContext, useWatch } from 'react-hook-form';
import { Link } from 'react-router-dom';
import type { OptionProps, SingleValueProps } from 'react-select';
import styled, { css, keyframes } from 'styled-components';

import { Alert } from 'design/Alert';
import Box from 'design/Box';
import { ButtonPrimary } from 'design/Button';
import ButtonIcon from 'design/ButtonIcon';
import Flex, { Stack } from 'design/Flex';
import { AmazonAws, Check, Refresh } from 'design/Icon';
import { Indicator } from 'design/Indicator';
import { LabelContent, LabelInput } from 'design/LabelInput/LabelInput';
import ExternalLink from 'design/Link';
import Text from 'design/Text';
import { HelperTextLine } from 'shared/components/FieldInput';
import Select from 'shared/components/Select';
import { TextSelectCopy } from 'shared/components/TextSelectCopy';

import type { TestInferenceModelRequest } from 'e-teleport/services/inference';
import { useTestInferenceModel } from 'e-teleport/services/inference/hooks';
import type { InferenceModelSchema } from 'e-teleport/SessionRecordings/setup/schema/model';
import cfg from 'teleport/config';
import { integrationService } from 'teleport/services/integrations';
import useStickyClusterId from 'teleport/useStickyClusterId';

interface Option {
  readonly arn: string;
  readonly label: string;
  readonly value: string;
}

interface IntegrationSelectorProps {
  helperTextId: string;
  isDisabled: boolean;
  isRefreshing: boolean;
  isRefreshDisabled: boolean;
  onAnimationIteration: () => void;
  onChange: (option: Option | null) => void;
  onRefresh: () => void;
  options: Option[];
  value: Option | null;
}

interface TestStatusIndicatorProps {
  confirmed: boolean;
  failedError: string | null;
  isPending: boolean;
  isSuccess: boolean;
  needsPermissions: boolean;
}

interface CloudShellInstructionsProps {
  command: string;
  confirmed: boolean;
  disabled: boolean;
  isPending: boolean;
  onCheckPermissions: () => void;
}

const ROLE_NAME_FROM_ARN = /arn:aws:iam::(\d+):role\/(.+)$/;

function createCloudShellCommand(roleArn: string): string {
  const match = ROLE_NAME_FROM_ARN.exec(roleArn);

  const role = match ? match[2] : roleArn;

  const params = new URLSearchParams({
    role,
    resource: '*',
  });

  const scriptUrl = `${cfg.baseUrl}/webapi/scripts/integrations/configure/aws-oidc-bedrock.sh?${params}`;

  return `bash -c "$(curl '${scriptUrl}')"`;
}

export function FieldTeleportAWSIntegration() {
  const [isSpinning, setIsSpinning] = useState(false);
  const [confirmed, setConfirmed] = useState(false);
  const [needsPermissions, setNeedsPermissions] = useState(false);
  const [selectedOption, setSelectedOption] = useState<Option | null>(null);
  const [failedError, setFailedError] = useState<string | null>(null);

  const helperTextId = useId();

  const prevRegionRef = useRef<string>(null);
  const prevModelRef = useRef<string>(null);

  const { clusterId } = useStickyClusterId();
  const { control, formState, getValues, setValue } =
    useFormContext<InferenceModelSchema>();

  const watchedValues = useWatch<InferenceModelSchema>({
    name: ['region', 'model'],
  });

  const region =
    typeof watchedValues[0] === 'string' ? watchedValues[0] : undefined;
  const model =
    typeof watchedValues[1] === 'string' ? watchedValues[1] : undefined;

  const integrations = useQuery({
    queryKey: ['integrations', 'aws', 'inference-models'],
    queryFn: () => integrationService.fetchIntegrations(),
  });

  const isLoading = integrations.isFetching;

  useEffect(() => {
    if (isLoading) {
      setIsSpinning(true);
    }
  }, [isLoading]);

  const handleAnimationIteration = useCallback(() => {
    if (!isLoading) {
      setIsSpinning(false);
    }
  }, [isLoading]);

  const options = useMemo<Option[]>(
    () =>
      integrations.data?.items
        ?.filter(integration => integration.kind === 'aws-oidc')
        .map(integration => ({
          arn: integration.spec.roleArn,
          label: integration.name,
          value: integration.name,
        })) ?? [],
    [integrations.data?.items]
  );

  const test = useTestInferenceModel({
    onError() {
      control._disableForm(false);
      setFailedError('Network error occurred during connection test');
    },
    onMutate() {
      control._disableForm(true);
      setFailedError(null);
      setConfirmed(false);
    },
    onSuccess(data) {
      control._disableForm(false);
      setNeedsPermissions(!data.success);

      if (data.success) {
        if (selectedOption) {
          setValue('integrationName', selectedOption.value, {
            shouldValidate: true,
          });
          setConfirmed(true);
        }
      } else {
        setFailedError(data.message ?? 'Connection test failed');
      }
    },
  });

  const testInferenceModel = useCallback(
    (option: Option) => {
      const values = getValues();
      const regionValue =
        'region' in values && typeof values.region === 'string'
          ? values.region
          : '';

      const request: TestInferenceModelRequest = {
        bedrock: {
          integration: option.value,
          modelId: values.model,
          region: regionValue,
        },
      };

      test.mutate({ clusterId, request });
    },
    [clusterId, getValues, test]
  );

  const handleChange = useCallback(
    (option: Option | null) => {
      setValue('integrationName', '', { shouldValidate: true });
      setSelectedOption(option);
      setNeedsPermissions(false);
      setConfirmed(false);
      setFailedError(null);

      if (option) {
        testInferenceModel(option);
      }
    },
    [setValue, testInferenceModel]
  );

  const handleCheckPermissions = useCallback(() => {
    if (selectedOption) {
      testInferenceModel(selectedOption);
    }
  }, [selectedOption, testInferenceModel]);

  // Re-test when region or model changes
  useEffect(() => {
    const regionChanged = prevRegionRef.current !== region;
    const modelChanged = prevModelRef.current !== model;

    prevRegionRef.current = region;
    prevModelRef.current = model;

    if ((regionChanged || modelChanged) && selectedOption && region && model) {
      setValue('integrationName', '', { shouldValidate: true });
      setNeedsPermissions(false);
      setConfirmed(false);
      setFailedError(null);
      testInferenceModel(selectedOption);
    }
  }, [region, model, selectedOption, setValue, testInferenceModel]);

  const permissionsCommand = useMemo(() => {
    if (!selectedOption || !region || !model) {
      return '';
    }
    return createCloudShellCommand(selectedOption.arn);
  }, [selectedOption, region, model]);

  const value =
    options.find(option => option.value === selectedOption?.value) ?? null;

  const isDisabled =
    !region ||
    !model ||
    test.isPending ||
    formState.disabled ||
    integrations.isPending;

  return (
    <Stack gap={3} width="100%">
      <IntegrationSelector
        helperTextId={helperTextId}
        isDisabled={isDisabled}
        isRefreshing={isSpinning}
        isRefreshDisabled={integrations.isPending}
        onAnimationIteration={handleAnimationIteration}
        onChange={handleChange}
        onRefresh={() => void integrations.refetch()}
        options={options}
        value={value}
      />

      <TestStatusIndicator
        confirmed={confirmed}
        failedError={failedError}
        isPending={test.isPending}
        isSuccess={test.isSuccess}
        needsPermissions={needsPermissions}
      />

      {needsPermissions && selectedOption && permissionsCommand && (
        <CloudShellInstructions
          command={permissionsCommand}
          confirmed={confirmed}
          disabled={formState.disabled ?? false}
          isPending={test.isPending}
          onCheckPermissions={handleCheckPermissions}
        />
      )}
    </Stack>
  );
}

function IntegrationSelector({
  helperTextId,
  isDisabled,
  isRefreshing,
  isRefreshDisabled,
  onAnimationIteration,
  onChange,
  onRefresh,
  options,
  value,
}: IntegrationSelectorProps) {
  return (
    <Flex width="100%" alignItems="center" gap={2}>
      <Box flex={1}>
        <LabelInput mb={0}>
          <LabelContent required={true} mb={1}>
            Teleport AWS Integration to use
          </LabelContent>

          <Select<Option>
            components={{
              SingleValue: IntegrationSingleValue,
              Option: IntegrationOption,
            }}
            isDisabled={isDisabled}
            onChange={onChange}
            options={options}
            value={value}
          />
        </LabelInput>

        <HelperTextLine
          hasError={false}
          helperTextId={helperTextId}
          helperText={
            <Link to="/web/integrations/new/aws-oidc" target="_blank">
              Add a new AWS integration
            </Link>
          }
        />
      </Box>

      <ButtonIcon
        type="button"
        disabled={isRefreshDisabled}
        mb={1}
        onClick={onRefresh}
      >
        <SpinningIcon
          isSpinning={isRefreshing}
          onAnimationIteration={onAnimationIteration}
        >
          <Refresh size="small" />
        </SpinningIcon>
      </ButtonIcon>
    </Flex>
  );
}

function TestStatusIndicator({
  confirmed,
  failedError,
  isPending,
  isSuccess,
  needsPermissions,
}: TestStatusIndicatorProps) {
  if (!needsPermissions && isPending) {
    return (
      <Flex alignItems="center" gap={2}>
        <Indicator delay="none" size="small" />
        <Text color="text.slightlyMuted">
          Checking if additional permissions are needed...
        </Text>
      </Flex>
    );
  }

  if (failedError && needsPermissions) {
    return (
      <Alert kind="outline-info" mb={0}>
        The connection test failed: {failedError}
      </Alert>
    );
  }

  if (failedError && !needsPermissions && !isPending) {
    return (
      <Alert kind="danger" mb={0}>
        {failedError}
      </Alert>
    );
  }

  if (isSuccess && confirmed) {
    return (
      <Alert kind="success" mb={0}>
        Permissions are correctly configured
      </Alert>
    );
  }

  return null;
}

function CloudShellInstructions({
  command,
  confirmed,
  disabled,
  isPending,
  onCheckPermissions,
}: CloudShellInstructionsProps) {
  return (
    <PermissionsBox $confirmed={confirmed}>
      <Flex justifyContent="space-between" alignItems="center" mb={2}>
        <Text typography="h4">CloudShell Instructions</Text>
        <AmazonAws size="large" />
      </Flex>

      <Text color="text.slightlyMuted" mb={2}>
        Run this command in{' '}
        <ExternalLink
          href="https://console.aws.amazon.com/cloudshell/home"
          target="_blank"
        >
          AWS CloudShell
        </ExternalLink>{' '}
        to set up the necessary permissions for Bedrock.
      </Text>

      <TextSelectCopy text={command} allowMultiline mb={3} />

      <Flex justifyContent="flex-end">
        <ButtonPrimary
          disabled={confirmed || disabled}
          onClick={onCheckPermissions}
          type="button"
        >
          {isPending ? (
            'Checking...'
          ) : confirmed ? (
            <>
              <Check size="small" mr={2} />
              Permissions verified
            </>
          ) : (
            'Check permissions'
          )}
        </ButtonPrimary>
      </Flex>
    </PermissionsBox>
  );
}

function IntegrationOption(props: OptionProps<Option>) {
  return (
    <OptionContainer {...props.innerProps}>
      <OptionLabel>{props.data.label}</OptionLabel>
      <OptionArn>{props.data.arn}</OptionArn>
    </OptionContainer>
  );
}

function IntegrationSingleValue(props: SingleValueProps<Option>) {
  return (
    <SingleValueContainer>
      <OptionLabel>{props.data.label}</OptionLabel>
      <OptionArn>{props.data.arn}</OptionArn>
    </SingleValueContainer>
  );
}

const spin = keyframes`
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
`;

const SpinningIcon = styled.span<{ isSpinning: boolean }>`
  display: inline-flex;
  ${p =>
    p.isSpinning &&
    css`
      animation: ${spin} 0.8s linear infinite;
    `}
`;

const PermissionsBox = styled(Box)<{ $confirmed: boolean }>`
  border: 1px solid ${p => p.theme.colors.interactive.tonal.primary[1]};
  border-radius: ${p => p.theme.radii[3]}px;
  padding: ${p => p.theme.space[3]}px;
  opacity: ${p => (p.$confirmed ? 0.5 : 1)};
`;

const OptionContainer = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: ${p => p.theme.space[2]}px ${p => p.theme.space[3]}px;
  width: 100%;
  cursor: pointer;
  box-sizing: border-box;

  &:hover {
    background: ${p => p.theme.colors.interactive.tonal.neutral[0]};
  }
`;

const OptionLabel = styled.span`
  font-size: 14px;
`;

const OptionArn = styled.span`
  font-size: 12px;
  color: ${p => p.theme.colors.text.muted};
`;

const SingleValueContainer = styled.div`
  display: flex;
  align-items: center;
  gap: 12px;
  grid-area: 1 / 1 / 2 / 3;
  margin-inline-end: 0.125rem;
`;
