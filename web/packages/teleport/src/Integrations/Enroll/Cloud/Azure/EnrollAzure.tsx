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

import { Box, ButtonSecondary, Flex, Subtitle1, Text } from 'design';
import FieldInput from 'shared/components/FieldInput';
import { InfoGuideContainer } from 'shared/components/SlidingSidePanel/InfoGuide';
import Validation from 'shared/components/Validation';
import {
  requiredField,
  requiredIntegrationName,
} from 'shared/components/Validation/rules';

import { SlidingSidePanel } from 'teleport/components/SlidingSidePanel/SlidingSidePanel';
import cfg from 'teleport/config';
import { Header } from 'teleport/Discover/Shared';
import { useNoMinWidth } from 'teleport/Main';
import { zIndexMap } from 'teleport/Navigation/zIndexMap';
import { AzureRegion, IntegrationKind } from 'teleport/services/integrations';
import { IntegrationEnrollKind } from 'teleport/services/userEvent/types';
import { useClusterVersion } from 'teleport/useClusterVersion';

import {
  CheckIntegrationButton,
  CircleNumber,
  Container,
  Divider,
  RegionsSection as BaseRegionsSection,
  RegionsSectionProps,
  useCloudIntegration,
} from '../Shared';
import {
  ContentWithSidePanel,
  InfoGuideSwitch,
  InfoGuideTitle,
  PANEL_WIDTH,
  TerraformInfoGuide,
  useTerraformInfoGuide,
} from '../Shared/InfoGuide';
import { RegionSelect } from '../Shared/RegionMultiSelector/RegionSelect';
import { DeploymentMethodSection } from './DeploymentMethodSection';
import { InfoGuideContent } from './InfoGuide';
import { Prerequisites } from './Prerequisites';
import { azureRegionGroups } from './regions';
import { ResourcesSection } from './ResourcesSection';
import { buildTerraformConfig } from './tf_module';
import { AzureScope, VmConfig, WildcardRegion } from './types';

export function EnrollAzure() {
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
  } = useCloudIntegration(IntegrationEnrollKind.AzureCloud);

  const [regions, setRegions] = useState<WildcardRegion | AzureRegion[]>([
    '*',
  ] as WildcardRegion);

  const [vmConfig, setVmConfig] = useState<VmConfig>({
    enabled: true,
    tags: [],
  });

  const [configScope, setConfigScope] = useState<AzureScope>({
    resource_group: '',
    managed_identity_region: 'eastus',
  });

  const terraformConfig = useMemo(
    () =>
      buildTerraformConfig({
        integrationName,
        version: clusterVersion,
        regions: regions,
        vmConfig: vmConfig,
        configScope: configScope,
      }),
    [integrationName, clusterVersion, vmConfig, regions, configScope]
  );

  const {
    isPanelOpen,
    setIsPanelOpen,
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
              <Header>Connect Azure</Header>
              <InfoGuideSwitch
                isPanelOpen={isPanelOpen}
                activeTab={activeInfoGuideTab}
                onSwitch={onInfoGuideClick}
              />
            </Flex>
            <Subtitle1 mb={3}>
              Connect your Azure account to automatically discover and enroll
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
              <ConfigurationScopeSection
                configScope={configScope}
                onChange={setConfigScope}
                disabled={isFetching}
              />
              <Divider />
              <ResourcesSection vmConfig={vmConfig} onVmChange={setVmConfig} />
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
                integrationKind={IntegrationKind.AzureOidc}
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
                      copyTerraformConfig(terraformConfig);
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
        Provide a name to identify this Azure integration in Teleport.
      </Text>
      <FieldInput
        ml={4}
        mb={0}
        rule={requiredIntegrationName}
        value={integrationName}
        required={true}
        label="Integration name"
        placeholder="my-azure-integration"
        maxWidth={360}
        disabled={disabled}
        onChange={e => onChange(e.target.value.trim())}
      />
    </>
  );
}

type ConfigurationScopeSectionProps = {
  configScope: AzureScope;
  onChange: (scope: AzureScope) => void;
  disabled?: boolean;
};

export function ConfigurationScopeSection({
  configScope,
  onChange,
  disabled = false,
}: ConfigurationScopeSectionProps) {
  return (
    <>
      <Flex alignItems="center" fontSize={4} fontWeight="medium" mb={3}>
        <CircleNumber>2</CircleNumber>
        Configuration Scope
      </Flex>
      <Text ml={4}>Single Azure Account</Text>
      <Text ml={4} mb={3} color="text.slightlyMuted">
        Discover resources from one specific Azure account. Additional accounts
        require separate integration setup. <br />
        Best for: Single-account environments or testing.
      </Text>
      <Box ml={4} borderColor="interactive.tonal.neutral.0">
        <Box
          pl={4}
          borderLeft="2px solid"
          borderColor="interactive.tonal.neutral.0"
        >
          <Text fontSize={2} mb={3}>
            IAM resources used for discovery in Teleport will be created using
            the account configured for your Azure Terraform provider.
          </Text>
        </Box>
      </Box>
      <FieldInput
        ml={4}
        mb={3}
        rule={requiredField('Resource group name is required')}
        value={configScope.resource_group}
        required={true}
        label="Azure Resource Group Name"
        placeholder="my-resource-group"
        maxWidth={400}
        disabled={disabled}
        onChange={e =>
          onChange({
            ...configScope,
            resource_group: e.target.value,
          })
        }
      />
      <Box ml={4} mb={0}>
        <RegionSelect
          isMulti={false}
          regionGroups={azureRegionGroups}
          selectedRegions={configScope.managed_identity_region}
          onChange={(region: string) =>
            onChange({
              ...configScope,
              managed_identity_region: region as AzureRegion,
            })
          }
          label="Azure Managed Identity Location"
          placeholder="Select region..."
          disabled={disabled}
          required={true}
          rule={requiredField('Managed identity location is required')}
        />
      </Box>
    </>
  );
}

export function RegionsSection(
  props: Omit<RegionsSectionProps<AzureRegion>, 'regionGroups'>
) {
  return (
    <BaseRegionsSection
      regions={props.regions}
      regionGroups={azureRegionGroups}
      onChange={props.onChange}
    />
  );
}
