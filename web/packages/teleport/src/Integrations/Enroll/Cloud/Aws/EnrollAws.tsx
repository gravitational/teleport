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
import { Info, Spinner } from 'design/Icon';
import { rotate360 } from 'design/keyframes';
import { ResourceIcon } from 'design/ResourceIcon';
import { HoverTooltip } from 'design/Tooltip';
import {
  ViewModeSwitchButton,
  ViewModeSwitchContainer,
} from 'shared/components/Controls/ViewModeSwitch';
import FieldInput from 'shared/components/FieldInput';
import { useInfoGuide } from 'shared/components/SlidingSidePanel/InfoGuide';
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

export function EnrollAws() {
  useNoMinWidth();

  const [integrationName, setIntegrationName] = useState('');

  const [regions, setRegions] = useState<WildcardRegion | AwsRegion[]>([
    '*',
  ] as WildcardRegion);

  const [ec2Config, setEc2Config] = useState<Ec2Config>({
    enabled: true,
    tags: [],
  });

  const terraformConfig = useMemo(
    () =>
      buildTerraformConfig({
        integrationName,
        regions,
        ec2Config,
      }),
    [integrationName, regions, ec2Config]
  );

  const copyConfigButtonRef = useRef<HTMLButtonElement>(null);

  const [activeInfoGuideSection, setActiveInfoGuideSection] = useState<
    'info' | 'terraform'
  >('terraform');

  const { infoGuideConfig: currentInfoGuideConfig, setInfoGuideConfig } =
    useInfoGuide();

  const infoGuideConfig = useMemo(
    () => ({
      guide:
        activeInfoGuideSection === 'terraform' ? (
          <TerraformInfoGuide
            terraformConfig={terraformConfig}
            copyConfigButtonRef={copyConfigButtonRef}
          />
        ) : (
          <InfoGuideContent />
        ),
      title: (
        <InfoGuideTitle
          activeSection={activeInfoGuideSection}
          onSectionChange={setActiveInfoGuideSection}
        />
      ),
      panelWidth: PANEL_WIDTH,
    }),
    [terraformConfig, activeInfoGuideSection]
  );

  useEffect(() => {
    setInfoGuideConfig(infoGuideConfig);
  }, [setInfoGuideConfig, infoGuideConfig]);

  const onInfoGuideClick = (section: 'info' | 'terraform') => {
    if (!!currentInfoGuideConfig && activeInfoGuideSection === section) {
      setInfoGuideConfig(null);
    } else {
      setActiveInfoGuideSection(section);
      setInfoGuideConfig(infoGuideConfig);
    }
  };

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

  return (
    <Validation>
      {({ validator }) => (
        <Flex pt={3}>
          <Box flex="1" mr={3}>
            <Flex justifyContent="space-between" alignItems="start" mb={1}>
              <Header>Connect Amazon Web Services</Header>
              <ViewModeSwitchContainer
                aria-label="Info Guide Mode Switch"
                aria-orientation="horizontal"
                role="radiogroup"
              >
                <HoverTooltip tipContent="Info Guide">
                  <ViewModeSwitchButton
                    className={
                      !!currentInfoGuideConfig &&
                      activeInfoGuideSection === 'info'
                        ? 'selected'
                        : ''
                    }
                    onClick={() => onInfoGuideClick('info')}
                    role="radio"
                    aria-checked={
                      !!currentInfoGuideConfig &&
                      activeInfoGuideSection === 'info'
                    }
                    aria-label="Info Guide"
                    first
                  >
                    <Info size="small" color="text.main" />
                  </ViewModeSwitchButton>
                </HoverTooltip>
                <HoverTooltip tipContent="Terraform Configuration">
                  <ViewModeSwitchButton
                    className={
                      !!currentInfoGuideConfig &&
                      activeInfoGuideSection === 'terraform'
                        ? 'selected'
                        : ''
                    }
                    onClick={() => onInfoGuideClick('terraform')}
                    role="radio"
                    aria-checked={
                      !!currentInfoGuideConfig &&
                      activeInfoGuideSection === 'terraform'
                    }
                    aria-label="Terraform Configuration"
                    last
                  >
                    <ResourceIcon name="terraform" width="16px" height="16px" />
                  </ViewModeSwitchButton>
                </HoverTooltip>
              </ViewModeSwitchContainer>
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
              />
            </Container>
            <Box mb={2}>
              <IntegrationButton
                integrationName={integrationName}
                integrationKind={IntegrationKind.AwsOidc}
                integrationExists={integrationExists}
                isPolling={isFetching}
                onClick={() => {
                  if (validator.validate()) {
                    checkIntegration();
                  }
                }}
              />
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
      <Flex alignItems="center" fontSize={4} fontWeight="medium" mb={3}>
        <CircleNumber>1</CircleNumber>
        Integration Details
      </Flex>
      <Text ml={4} mb={3}>
        Provide a name to identify this AWS integration in Teleport.
      </Text>
      <FieldInput
        ml={4}
        mb={2}
        autoFocus={true}
        rule={requiredIntegrationName}
        value={integrationName}
        required={true}
        label="Integration name"
        placeholder="Integration Name"
        maxWidth={360}
        disabled={disabled}
        onChange={e => onChange(e.target.value.trim())}
      />
    </>
  );
}

function ConfigurationScopeSection() {
  return (
    <>
      <Flex alignItems="center" fontSize={4} fontWeight="medium" mb={3}>
        <CircleNumber>2</CircleNumber>
        Configuration Scope
      </Flex>
      <Text ml={4}>Single AWS Account</Text>
      <Text ml={4} mb={3} color="text.slightlyMuted">
        Discover resources from one specific AWS account. Additional accounts
        require separate integration setup. Best for: Single-account
        environments or testing.
      </Text>
      <Box ml={4} borderColor="interactive.tonal.neutral.0">
        <Text fontSize={2}>
          Teleport will automatically detect your AWS account when you deploy
          the IAM role.
        </Text>
      </Box>
    </>
  );
}

type IntegrationButtonProps = {
  isPolling: boolean;
  integrationName: string;
  onClick: () => void;
  integrationExists: boolean;
  integrationKind?: IntegrationKind;
};

const IntegrationButton = ({
  isPolling,
  integrationName,
  onClick,
  integrationExists,
  integrationKind = IntegrationKind.AwsOidc,
}: IntegrationButtonProps) => {
  if (integrationExists && integrationName) {
    return (
      <ButtonPrimary
        as={InternalLink}
        to={cfg.getIaCIntegrationRoute(integrationKind, integrationName)}
      >
        View Integration
      </ButtonPrimary>
    );
  }

  return (
    <ButtonPrimary disabled={isPolling} onClick={onClick} gap={2}>
      {isPolling && <AnimatedSpinner size="small" />}
      {isPolling ? 'Checking...' : 'Check Integration'}
    </ButtonPrimary>
  );
};

const Container = styled(Flex)`
  border-radius: 8px;
  background: ${props => props.theme.colors.levels.elevated};

  box-shadow:
    0 2px 1px -1px rgba(0, 0, 0, 0.2),
    0 1px 1px 0 rgba(0, 0, 0, 0.14),
    0 1px 3px 0 rgba(0, 0, 0, 0.12);
`;

const AnimatedSpinner = styled(Spinner)`
  animation: ${rotate360} 1.5s linear infinite;
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
  margin-top: ${p => p.theme.space[4]}px;
  margin-bottom: ${p => p.theme.space[4]}px;
  border: 1px solid ${p => p.theme.colors.interactive.tonal.neutral[0]};
  width: 100%;
`;
