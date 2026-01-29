/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { useQuery } from '@tanstack/react-query';
import { useEffect, useMemo, useRef, useState } from 'react';
import { Link as InternalLink } from 'react-router-dom';
import styled from 'styled-components';

import {
  Box,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  Subtitle1,
  Text,
} from 'design';
import { Info } from 'design/Icon';
import { ResourceIcon } from 'design/ResourceIcon';
import { HoverTooltip } from 'design/Tooltip';
import {
  ViewModeSwitchButton,
  ViewModeSwitchContainer,
} from 'shared/components/Controls/ViewModeSwitch';
import FieldInput from 'shared/components/FieldInput';
import {
  InfoGuideConfig,
  useInfoGuide,
} from 'shared/components/SlidingSidePanel/InfoGuide';
import { useToastNotifications } from 'shared/components/ToastNotification';
import Validation from 'shared/components/Validation';
import { requiredIntegrationName } from 'shared/components/Validation/rules';

import cfg from 'teleport/config';
import { Header } from 'teleport/Discover/Shared';
import { useNoMinWidth } from 'teleport/Main';
import { ApiError } from 'teleport/services/api/parseError';
import {
  Regions as AwsRegion,
  IntegrationKind,
  integrationService,
} from 'teleport/services/integrations';
import { useClusterVersion } from 'teleport/useClusterVersion';

import { DeploymentMethodSection } from './DeploymentMethodSection';
import {
  InfoGuideContent,
  InfoGuideTitle,
  PANEL_WIDTH,
  TerraformInfoGuide,
} from './InfoGuide';
import { Prerequisites } from './Prerequisites';
import { RegionsSection } from './RegionsSection';
import { ResourcesSection } from './ResourcesSection';
import { buildTerraformConfig } from './tf_module';
import { Ec2Config, WildcardRegion } from './types';

const INTEGRATION_CHECK_RETRIES = 6;
const INTEGRATION_CHECK_RETRY_DELAY = 5000;

export type InfoGuideTab = 'info' | 'terraform' | null;

export function EnrollAws() {
  useNoMinWidth();

  const { clusterVersion } = useClusterVersion();

  const [integrationName, setIntegrationName] = useState('');

  const [regions, setRegions] = useState<WildcardRegion | AwsRegion[]>([
    '*',
  ] as WildcardRegion);

  const [ec2Config, setEc2Config] = useState<Ec2Config>({
    enabled: true,
    tags: [],
  });

  const [configCopied, setConfigCopied] = useState(false);

  const handleConfigCopy = () => {
    setConfigCopied(true);
    setTimeout(() => {
      setConfigCopied(false);
    }, 1000);
  };

  const terraformConfig = useMemo(
    () =>
      buildTerraformConfig({
        integrationName,
        regions,
        ec2Config,
        version: clusterVersion,
      }),
    [integrationName, regions, ec2Config, clusterVersion]
  );

  const copyConfigButtonRef = useRef<HTMLButtonElement>(null);

  const [activeInfoGuideTab, setActiveInfoGuideTab] =
    useState<InfoGuideTab>('terraform');

  const { infoGuideConfig: currentInfoGuideConfig, setInfoGuideConfig } =
    useInfoGuide();

  const infoGuideConfig = useMemo(
    () => ({
      guide:
        activeInfoGuideTab === 'terraform' ? (
          <TerraformInfoGuide
            terraformConfig={terraformConfig}
            copyConfigButtonRef={copyConfigButtonRef}
            configCopied={configCopied}
          />
        ) : (
          <InfoGuideContent />
        ),
      title: (
        <InfoGuideTitle
          activeSection={activeInfoGuideTab}
          onSectionChange={setActiveInfoGuideTab}
        />
      ),
      panelWidth: PANEL_WIDTH,
    }),
    [terraformConfig, activeInfoGuideTab, configCopied]
  );

  useEffect(() => {
    setInfoGuideConfig(infoGuideConfig);
  }, [setInfoGuideConfig, infoGuideConfig]);

  const toastNotifications = useToastNotifications();
  const didShowToast = useRef(false);

  const integrationQueryKey = ['integration', integrationName];

  const {
    data: integrationData,
    isFetching,
    isSuccess,
    isError,
    refetch,
  } = useQuery({
    queryKey: integrationQueryKey,
    queryFn: () => integrationService.fetchIntegration(integrationName),
    enabled: false,
    retry: (failureCount, error: unknown) => {
      const shouldRetry =
        failureCount < INTEGRATION_CHECK_RETRIES &&
        error instanceof ApiError &&
        error.response.status === 404;
      return shouldRetry;
    },
    retryDelay: INTEGRATION_CHECK_RETRY_DELAY,
    gcTime: 0,
  });

  const shouldShowErrorToast = isError && !isFetching && integrationName;
  const shouldShowSuccessToast = isSuccess && !isFetching && integrationName;

  // handle success / error toasts
  useEffect(() => {
    if (isFetching) {
      return;
    }

    if (shouldShowSuccessToast) {
      toastNotifications.add({
        severity: 'success',
        content: {
          title: 'Amazon Web Services successfully added',
          description:
            'Amazon Web Services has been successfully added ' +
            'to this Teleport Cluster. Your resources will appear ' +
            "automatically as they're discovered. This may take a few minutes.",
          action: {
            content: 'View Integration',
            linkTo: cfg.getIaCIntegrationRoute(
              IntegrationKind.AwsOidc,
              integrationName
            ),
          },
          isAutoRemovable: false,
        },
      });
    } else if (shouldShowErrorToast) {
      toastNotifications.add({
        severity: 'error',
        content: {
          title: 'Failed to detect integration',
          description: `Unable to detect the AWS integration "${integrationName}". Please check your configuration and try again.`,
        },
      });
    }

    didShowToast.current = true;
  }, [isFetching]);

  const checkIntegration = () => {
    didShowToast.current = false;
    refetch();
  };

  const integrationExists = !!integrationData;

  const onInfoGuideClick = (section: InfoGuideTab) => {
    if (!!currentInfoGuideConfig && activeInfoGuideTab === section) {
      setInfoGuideConfig(null);
    } else {
      setActiveInfoGuideTab(section);
      setInfoGuideConfig(infoGuideConfig);
    }
  };

  return (
    <Validation>
      {({ validator }) => (
        <Flex pt={3}>
          <Box flex="1" mr={3}>
            <Flex justifyContent="space-between" alignItems="start" mb={1}>
              <Header>Connect Amazon Web Services</Header>
              <InfoGuideSwitch
                currentConfig={currentInfoGuideConfig}
                activeTab={activeInfoGuideTab}
                onSwitch={onInfoGuideClick}
              />
            </Flex>
            <Subtitle1 mb={3}>
              Connect your AWS account to automatically discover and enroll
              resources in your Teleport Cluster.
            </Subtitle1>
            <Container flexDirection="column" p={4} mb={4}>
              <Prerequisites />
            </Container>
            <Container flexDirection="column" p={4} mb={3}>
              <IntegrationSection
                integrationName={integrationName}
                onChange={setIntegrationName}
                disabled={isFetching}
              />
              <Divider />
              <ConfigurationScopeSection />
              <Divider />
              <ResourcesSection
                ec2Config={ec2Config}
                onEc2Change={setEc2Config}
              />
              <Divider />
              <RegionsSection regions={regions} onChange={setRegions} />
              <Divider />
              <DeploymentMethodSection
                terraformConfig={terraformConfig}
                copyConfigButtonRef={copyConfigButtonRef}
                integrationExists={integrationExists}
                integrationName={integrationName}
                onCheckIntegration={() => {
                  if (validator.validate()) {
                    checkIntegration();
                  }
                }}
                isCheckingIntegration={isFetching}
                configCopied={configCopied}
                onConfigCopy={handleConfigCopy}
              />
            </Container>
            <Box mb={2}>
              <ButtonPrimary
                as={
                  integrationExists && integrationName
                    ? InternalLink
                    : undefined
                }
                to={
                  integrationExists && integrationName
                    ? cfg.getIaCIntegrationRoute(
                        IntegrationKind.AwsOidc,
                        integrationName
                      )
                    : undefined
                }
                disabled={!integrationExists || !integrationName}
                gap={2}
              >
                View Integration
              </ButtonPrimary>
              <ButtonSecondary
                ml={3}
                as={InternalLink}
                to={cfg.getIntegrationEnrollRoute(null)}
              >
                Back
              </ButtonSecondary>
            </Box>
          </Box>
        </Flex>
      )}
    </Validation>
  );
}

type IntegrationSectionProps = {
  integrationName: string;
  onChange: (name: string) => void;
  disabled: boolean;
};

export function IntegrationSection({
  integrationName,
  onChange,
  disabled = false,
}: IntegrationSectionProps) {
  return (
    <>
      <Flex alignItems="center" fontSize={4} fontWeight="medium" mb={1}>
        <CircleNumber>1</CircleNumber>
        Integration Details
      </Flex>
      <Text ml={4} mb={3}>
        Provide a name to identify this AWS integration in Teleport.
      </Text>
      <FieldInput
        ml={4}
        mb={0}
        autoFocus={true}
        rule={requiredIntegrationName}
        value={integrationName}
        required={true}
        label="Integration name"
        placeholder="my-aws-integration"
        maxWidth={360}
        disabled={disabled}
        onChange={e => onChange(e.target.value.trim())}
      />
    </>
  );
}

export function ConfigurationScopeSection() {
  return (
    <>
      <Flex alignItems="center" fontSize={4} fontWeight="medium" mb={3}>
        <CircleNumber>2</CircleNumber>
        Configuration Scope
      </Flex>
      <Text ml={4}>Single AWS Account</Text>
      <Text ml={4} mb={3} color="text.slightlyMuted">
        Discover resources from one specific AWS account. Additional accounts
        require separate integration setup. <br />
        Best for: Single-account environments or testing.
      </Text>
      <Box ml={4} borderColor="interactive.tonal.neutral.0">
        <Box
          pl={4}
          borderLeft="2px solid"
          borderColor="interactive.tonal.neutral.0"
        >
          <Text fontSize={2}>
            IAM resources used for discovery in Teleport will be created using
            the account configured for your AWS Terraform provider.
          </Text>
        </Box>
      </Box>
    </>
  );
}

type InfoGuideSwitchProps = {
  activeTab: InfoGuideTab;
  currentConfig: InfoGuideConfig | null;
  onSwitch: (activeTab: InfoGuideTab) => void;
};

export const InfoGuideSwitch = ({
  activeTab,
  currentConfig,
  onSwitch,
}: InfoGuideSwitchProps) => {
  return (
    <ViewModeSwitchContainer
      aria-label="Info Guide Mode Switch"
      aria-orientation="horizontal"
      role="radiogroup"
    >
      <HoverTooltip tipContent="Info Guide">
        <ViewModeSwitchButton
          className={!!currentConfig && activeTab === 'info' ? 'selected' : ''}
          onClick={() => onSwitch('info')}
          role="radio"
          aria-checked={!!currentConfig && activeTab === 'info'}
          aria-label="Info Guide"
          first
        >
          <Info size="small" color="text.main" />
        </ViewModeSwitchButton>
      </HoverTooltip>
      <HoverTooltip tipContent="Terraform Configuration">
        <ViewModeSwitchButton
          className={
            !!currentConfig && activeTab === 'terraform' ? 'selected' : ''
          }
          onClick={() => onSwitch('terraform')}
          role="radio"
          aria-checked={!!currentConfig && activeTab === 'terraform'}
          aria-label="Terraform Configuration"
          last
        >
          <ResourceIcon name="terraform" width="16px" height="16px" />
        </ViewModeSwitchButton>
      </HoverTooltip>
    </ViewModeSwitchContainer>
  );
};

export const Container = styled(Flex)`
  border-radius: 8px;
  background: ${props => props.theme.colors.levels.elevated};

  box-shadow:
    0 2px 1px -1px rgba(0, 0, 0, 0.2),
    0 1px 1px 0 rgba(0, 0, 0, 0.14),
    0 1px 3px 0 rgba(0, 0, 0, 0.12);
`;

export const CircleNumber = styled.span`
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: ${p => p.theme.space[3]}px;
  height: ${p => p.theme.space[3]}px;
  border: 1px solid ${p => p.theme.colors.text.main};
  color: ${p => p.theme.colors.text.main};
  border-radius: 50%;
  font-size: 12px;
  font-weight: 500;
  margin-right: ${p => p.theme.space[2]}px;
  flex-shrink: 0;
  box-sizing: border-box;
`;

export const Divider = styled.hr`
  margin-top: ${p => p.theme.space[3]}px;
  margin-bottom: ${p => p.theme.space[3]}px;
  border: 1px solid ${p => p.theme.colors.interactive.tonal.neutral[0]};
  width: 100%;
`;
