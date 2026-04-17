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
import { Link as InternalLink } from 'react-router-dom';

import { Box, ButtonSecondary, Flex, Subtitle1, Text } from 'design';
import FieldInput from 'shared/components/FieldInput';
import Validation from 'shared/components/Validation';
import { requiredIntegrationName } from 'shared/components/Validation/rules';

import cfg from 'teleport/config';
import { Header } from 'teleport/Discover/Shared';
import { useNoMinWidth } from 'teleport/Main';
import {
  Regions as AwsRegion,
  IntegrationKind,
} from 'teleport/services/integrations';
import { IntegrationEnrollKind } from 'teleport/services/userEvent/types';
import { useClusterVersion } from 'teleport/useClusterVersion';

import {
  CheckIntegrationButton,
  CircleNumber,
  Container,
  Divider,
  WildcardRegion,
  useEnrollCloudIntegration,
} from '../Shared';
import {
  ContentWithSidePanel,
  InfoGuideSwitch,
  PANEL_WIDTH,
  TerraformInfoGuide,
  TerraformInfoGuideSidePanel,
  useTerraformInfoGuide,
} from '../Shared/InfoGuide';
import { DeploymentMethodSection } from './DeploymentMethodSection';
import { InfoGuideContent } from './InfoGuide';
import { Prerequisites } from './Prerequisites';
import { RegionsSection } from './RegionsSection';
import { ResourcesSection } from './ResourcesSection';
import { buildTerraformConfig } from './tf_module';
import { Ec2Config } from './types';

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

  const [regions, setRegions] = useState<WildcardRegion | AwsRegion[]>(['*']);

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

  const {
    isPanelOpen,
    activeInfoGuideTab,
    setActiveInfoGuideTab,
    onInfoGuideClick,
  } = useTerraformInfoGuide();

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
              <CheckIntegrationButton
                integrationExists={integrationExists}
                integrationName={integrationName}
                integrationKind={IntegrationKind.AwsOidc}
              />
              <ButtonSecondary
                ml={3}
                as={InternalLink}
                to={cfg.getIntegrationEnrollRoute(null)}
              >
                Back
              </ButtonSecondary>
            </Box>
          </ContentWithSidePanel>

          <TerraformInfoGuideSidePanel
            panelWidth={PANEL_WIDTH}
            activeTab={activeInfoGuideTab}
            onTabChange={setActiveInfoGuideTab}
            InfoGuideContent={<InfoGuideContent />}
            TerraformContent={
              <TerraformInfoGuide
                terraformConfig={terraformConfig}
                handleCopy={() => {
                  if (validator.validate() && terraformConfig) {
                    copyTerraformConfig(terraformConfig);
                  }
                }}
              />
            }
          />
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
