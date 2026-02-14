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

import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import { Link as InternalLink } from 'react-router';
import styled from 'styled-components';

import {
  Box,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  Subtitle1,
  Text,
} from 'design';
import { copyToClipboard } from 'design/utils/copyToClipboard';
import FieldInput from 'shared/components/FieldInput';
import { InfoGuideContainer } from 'shared/components/SlidingSidePanel/InfoGuide';
import { useToastNotifications } from 'shared/components/ToastNotification';
import Validation from 'shared/components/Validation';
import { requiredIntegrationName } from 'shared/components/Validation/rules';

import { SlidingSidePanel } from 'teleport/components/SlidingSidePanel/SlidingSidePanel';
import cfg from 'teleport/config';
import { Header } from 'teleport/Discover/Shared';
import { useNoMinWidth } from 'teleport/Main';
import { zIndexMap } from 'teleport/Navigation/zIndexMap';
import { ApiError } from 'teleport/services/api/parseError';
import {
  Regions as AwsRegion,
  IntegrationKind,
  integrationService,
} from 'teleport/services/integrations';
import { useClusterVersion } from 'teleport/useClusterVersion';

import { DeploymentMethodSection } from './DeploymentMethodSection';
import {
  ContentWithSidePanel,
  InfoGuideContent,
  InfoGuideSwitch,
  InfoGuideTab,
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

  const { clusterVersion } = useClusterVersion();

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
        version: clusterVersion,
      }),
    [integrationName, regions, ec2Config, clusterVersion]
  );

  const [isPanelOpen, setIsPanelOpen] = useState(true);
  const [activeInfoGuideTab, setActiveInfoGuideTab] =
    useState<InfoGuideTab>('terraform');

  const toastNotifications = useToastNotifications();

  const integrationQueryKey = ['integration', integrationName];

  const {
    data: integrationData,
    isFetching,
    isSuccess,
    isError,
    refetch,
  } = useQuery({
    queryKey: integrationQueryKey,
    queryFn: ({ signal }) =>
      integrationService.fetchIntegration(integrationName, signal),
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

  const queryClient = useQueryClient();

  // show success toast
  useEffect(() => {
    if (isSuccess && !isFetching && integrationName) {
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
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isSuccess, isFetching, integrationName]);

  // show error toast
  useEffect(() => {
    if (isError && !isFetching && integrationName) {
      toastNotifications.add({
        severity: 'error',
        content: {
          title: 'Failed to detect integration',
          description: `Unable to detect the AWS integration "${integrationName}". Please check your configuration and try again.`,
        },
      });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isError, isFetching, integrationName]);

  const checkIntegration = () => {
    refetch();
  };

  const integrationExists = !!integrationData;

  const onInfoGuideClick = (section: InfoGuideTab) => {
    if (isPanelOpen && activeInfoGuideTab === section) {
      setIsPanelOpen(false);
    } else {
      setActiveInfoGuideTab(section);
      setIsPanelOpen(true);
    }
  };

  return (
    <Validation>
      {({ validator }) => (
        <Box pt={3}>
          <ContentWithSidePanel
            isPanelOpen={isPanelOpen}
            panelWidth={PANEL_WIDTH}
          >
            <Flex justifyContent="space-between" alignItems="start" mb={1}>
              <Header>Connect Amazon Web Services</Header>
              <InfoGuideSwitch
                isPanelOpen={isPanelOpen}
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
                handleCopy={() => {
                  if (validator.validate() && terraformConfig) {
                    copyToClipboard(terraformConfig);
                  }
                }}
                integrationExists={integrationExists}
                integrationName={integrationName}
                handleCheckIntegration={() => {
                  if (validator.validate()) {
                    checkIntegration();
                  }
                }}
                handleCancelCheckIntegration={() => {
                  queryClient.cancelQueries({ queryKey: integrationQueryKey });
                  queryClient.resetQueries({ queryKey: integrationQueryKey });
                }}
                isCheckingIntegration={isFetching}
                checkIntegrationError={isError}
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
          </ContentWithSidePanel>

          <SlidingSidePanel
            isVisible={isPanelOpen}
            skipAnimation={false}
            panelWidth={PANEL_WIDTH}
            zIndex={zIndexMap.infoGuideSidePanel}
            slideFrom="right"
          >
            <InfoGuideContainer
              onClose={() => setIsPanelOpen(false)}
              title={
                <InfoGuideTitle
                  activeSection={activeInfoGuideTab}
                  onSectionChange={setActiveInfoGuideTab}
                />
              }
            >
              {activeInfoGuideTab === 'terraform' ? (
                <TerraformInfoGuide
                  terraformConfig={terraformConfig}
                  handleCopy={() => {
                    if (validator.validate() && terraformConfig) {
                      copyToClipboard(terraformConfig);
                    }
                  }}
                />
              ) : (
                <InfoGuideContent />
              )}
            </InfoGuideContainer>
          </SlidingSidePanel>
        </Box>
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
