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

import { useMemo, useState } from 'react';
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
import FieldInput from 'shared/components/FieldInput';
import { InfoGuideContainer } from 'shared/components/SlidingSidePanel/InfoGuide';
import Validation from 'shared/components/Validation';
import { requiredIntegrationName } from 'shared/components/Validation/rules';

import { SlidingSidePanel } from 'teleport/components/SlidingSidePanel/SlidingSidePanel';
import cfg from 'teleport/config';
import { Header } from 'teleport/Discover/Shared';
import { useNoMinWidth } from 'teleport/Main';
import { zIndexMap } from 'teleport/Navigation/zIndexMap';
import { IntegrationKind } from 'teleport/services/integrations';
import { IntegrationEnrollKind } from 'teleport/services/userEvent/types';
import { useClusterVersion } from 'teleport/useClusterVersion';

import { useEnrollCloudIntegration } from '../Shared';
import { DeploymentMethodSection } from './DeploymentMethodSection';
import {
  ContentWithSidePanel,
  InfoGuideContent,
  InfoGuideSwitch,
  InfoGuideTab,
  InfoGuideTitle,
  PANEL_WIDTH,
  responsivePanelWidth,
  TerraformInfoGuide,
} from './InfoGuide';
import { Prerequisites } from './Prerequisites';
import { ResourcesSection } from './ResourcesSection';
import { buildTerraformConfig } from './tf_module';
import {
  ServiceConfig,
  ServiceConfigs,
  ServiceType,
  WildcardRegion,
} from './types';

export function EnrollAws() {
  useNoMinWidth();

  const { clusterVersion } = useClusterVersion();

  const {
    integrationName,
    setIntegrationName,
    copyTerraformConfig,
    integrationExists,
    isFetching,
    isError,
    checkIntegration,
    cancelCheckIntegration,
  } = useEnrollCloudIntegration(IntegrationEnrollKind.AwsCloud);

  const [configs, setConfigs] = useState<ServiceConfigs>({
    ec2: { enabled: true, regions: ['*'] as WildcardRegion, tags: [] },
    eks: { enabled: false, regions: [], tags: [] },
  });

  const updateConfig = (type: ServiceType, config: ServiceConfig) => {
    setConfigs(prev => ({ ...prev, [type]: config }));
  };

  const terraformConfig = useMemo(
    () =>
      buildTerraformConfig({
        integrationName,
        configs,
        version: clusterVersion,
      }),
    [integrationName, configs, clusterVersion]
  );

  const [isPanelOpen, setIsPanelOpen] = useState(true);
  const [activeInfoGuideTab, setActiveInfoGuideTab] =
    useState<InfoGuideTab>('terraform');

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
          <FlexibleContent isPanelOpen={isPanelOpen} panelWidth={PANEL_WIDTH}>
            <Flex justifyContent="space-between" alignItems="center" mb={1}>
              <Header>Connect Amazon Web Services</Header>
              <Box mt={1}>
                <InfoGuideSwitch
                  isPanelOpen={isPanelOpen}
                  activeTab={activeInfoGuideTab}
                  onSwitch={onInfoGuideClick}
                />
              </Box>
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
              <ResourcesSection
                configs={configs}
                onConfigChange={updateConfig}
              />
              <Divider />
              <DeploymentMethodSection
                terraformConfig={terraformConfig}
                handleCopy={() => {
                  if (validator.validate() && terraformConfig) {
                    copyTerraformConfig(terraformConfig);
                  }
                }}
                integrationExists={integrationExists}
                integrationName={integrationName}
                handleCheckIntegration={() => {
                  if (validator.validate()) {
                    checkIntegration();
                  }
                }}
                handleCancelCheckIntegration={cancelCheckIntegration}
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
          </FlexibleContent>

          <FlexibleSidePanel
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
                      copyTerraformConfig(terraformConfig);
                    }
                  }}
                />
              ) : (
                <InfoGuideContent />
              )}
            </InfoGuideContainer>
          </FlexibleSidePanel>
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

const FlexibleContent = styled(ContentWithSidePanel)`
  && {
    margin-right: ${p => (p.isPanelOpen ? responsivePanelWidth : '0')};
  }
`;

const FlexibleSidePanel = styled(SlidingSidePanel)`
  && {
    width: ${responsivePanelWidth};
  }
`;
