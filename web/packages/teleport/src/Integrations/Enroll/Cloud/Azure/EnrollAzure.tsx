/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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
import { RadioGroup } from 'design/RadioGroup';
import { copyToClipboard } from 'design/utils/copyToClipboard';
import FieldInput from 'shared/components/FieldInput';
import Validation from 'shared/components/Validation';
import {
  requiredAzureSubscriptionId,
  requiredField,
  requiredIntegrationName,
} from 'shared/components/Validation/rules';

import cfg from 'teleport/config';
import { Header } from 'teleport/Discover/Shared';
import { useNoMinWidth } from 'teleport/Main';
import { IntegrationKind } from 'teleport/services/integrations';
import { IntegrationEnrollKind } from 'teleport/services/userEvent/types';
import { useClusterVersion } from 'teleport/useClusterVersion';

import {
  CheckIntegrationButton,
  CircleNumber,
  Container,
  Divider,
  useEnrollCloudIntegration,
} from '../Shared';
import {
  TerraformInfoGuideSidePanel,
  InfoGuideSwitch,
  useTerraformInfoGuide,
  ContentWithSidePanel,
  TerraformInfoGuide,
} from '../Shared/InfoGuide';
import { RegionSelect } from '../Shared/RegionSelect';
import { ApplyTerraformSection } from './ApplyTerraformSection';
import { InfoGuideContent } from './InfoGuide';
import { azureRegionOptionGroups, azureRegionOptions } from './regions';
import { ResourcesSection } from './ResourcesSection';
import { buildTerraformConfig } from './tf_module';
import { AzureManagedIdentity, VmConfig } from './types';

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
  } = useEnrollCloudIntegration(IntegrationEnrollKind.AzureCloud);

  const [vmConfig, setVmConfig] = useState<VmConfig>({
    type: 'vm',
    enabled: true,
    regions: ['*'],
    subscriptions: [],
    resourceGroups: [],
    tags: [],
  });

  const [managedIdentity, setManagedIdentity] = useState<AzureManagedIdentity>({
    region: 'eastus',
    resourceGroup: '',
    scope: 'managementGroup',
    managementGroupId: '',
  });

  const terraformConfig = useMemo(
    () =>
      buildTerraformConfig({
        integrationName,
        version: clusterVersion,
        vmConfig: vmConfig,
        managedIdentity: managedIdentity,
      }),
    [integrationName, clusterVersion, vmConfig, managedIdentity]
  );

  const isManagementGroupScope = managedIdentity.scope === 'managementGroup';

  const {
    isPanelOpen,
    activeInfoGuideTab,
    setActiveInfoGuideTab,
    onInfoGuideClick,
  } = useTerraformInfoGuide('info');

  return (
    <Validation>
      {({ validator }) => (
        <Box pt={3}>
          <ContentWithSidePanel isPanelOpen={isPanelOpen}>
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
            <Container flexDirection="column" p={4} mb={3}>
              <IntegrationSection
                integrationName={integrationName}
                onChange={setIntegrationName}
                disabled={isFetching}
              />
              <Divider />
              <ManagedIdentitySection
                managedIdentity={managedIdentity}
                onChange={setManagedIdentity}
                vmConfig={vmConfig}
                onVmChange={setVmConfig}
                disabled={isFetching}
              />
              <Divider />
              <ResourcesSection
                vmConfig={vmConfig}
                onVmChange={setVmConfig}
                allowWildcardSubscriptions={isManagementGroupScope}
              />
              <Divider />
              <ApplyTerraformSection
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

          <TerraformInfoGuideSidePanel
            activeTab={activeInfoGuideTab}
            onTabChange={setActiveInfoGuideTab}
            InfoGuideContent={<InfoGuideContent />}
            TerraformContent={
              <TerraformInfoGuide
                terraformConfig={terraformConfig}
                handleCopy={() => {
                  if (validator.validate() && terraformConfig) {
                    copyToClipboard(terraformConfig);
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

type ManagedIdentitySectionProps = {
  managedIdentity: AzureManagedIdentity;
  onChange: (identity: AzureManagedIdentity) => void;
  vmConfig: VmConfig;
  onVmChange: (config: VmConfig) => void;
  disabled?: boolean;
};

export function ManagedIdentitySection({
  managedIdentity,
  onChange,
  vmConfig,
  onVmChange,
  disabled = false,
}: ManagedIdentitySectionProps) {
  const isManagementGroupScope = managedIdentity.scope === 'managementGroup';

  return (
    <>
      <Flex alignItems="center" fontSize={4} fontWeight="medium" mb={1}>
        <CircleNumber>2</CircleNumber>
        Azure Managed Identity
      </Flex>
      <Text ml={4} mb={3}>
        Configure access for the managed identity used by the Discovery Service
        and where it is deployed.
      </Text>

      <Text ml={4} bold>
        Scope
      </Text>
      <Text ml={4} mb={1} fontSize="small" color="text.slightlyMuted">
        Configure the scope for the managed identity's role assignments. Use a
        root Management Group to discover resources across all child
        subscriptions or specific subscriptions.
      </Text>
      <Box ml={4} mb={3}>
        <RadioGroup
          name="azureScope"
          options={[
            { value: 'managementGroup', label: 'Management Group' },
            { value: 'subscription', label: 'Subscription' },
          ]}
          value={managedIdentity.scope}
          size="small"
          onChange={value =>
            onChange({
              ...managedIdentity,
              scope: value as AzureManagedIdentity['scope'],
              ...(value === 'managementGroup'
                ? { managementGroupId: managedIdentity.managementGroupId ?? '' }
                : { managementGroupId: undefined }),
            })
          }
        />
      </Box>
      {isManagementGroupScope ? (
        <FieldInput
          ml={4}
          mb={3}
          rule={requiredField('Management Group ID is required')}
          value={managedIdentity.managementGroupId || ''}
          required
          label="Management Group ID"
          placeholder="my-management-group-id"
          maxWidth={400}
          disabled={disabled}
          onChange={e =>
            onChange({
              ...managedIdentity,
              managementGroupId: e.target.value,
            })
          }
        />
      ) : (
        <FieldInput
          ml={4}
          mb={3}
          rule={requiredAzureSubscriptionId}
          value={vmConfig.subscriptions[0] || ''}
          required
          label="Subscription ID"
          placeholder="11111111-2222-3333-4444-555555555555"
          maxWidth={400}
          disabled={disabled}
          onChange={e =>
            onVmChange({
              ...vmConfig,
              subscriptions: [
                e.target.value,
                ...vmConfig.subscriptions.slice(1),
              ],
            })
          }
        />
      )}

      <Box ml={4} mb={0} maxWidth={400}>
        <RegionSelect
          isMulti={false}
          options={azureRegionOptionGroups}
          value={azureRegionOptions.find(
            opt => opt.value === managedIdentity.region
          )}
          onChange={option =>
            option &&
            onChange({
              ...managedIdentity,
              region: option.value,
            })
          }
          label="Location"
          placeholder="Select region..."
          isDisabled={disabled}
          required={true}
          rule={requiredField('Managed identity location is required')}
        />
      </Box>
      <FieldInput
        ml={4}
        mb={3}
        rule={requiredField('Resource group name is required')}
        value={managedIdentity.resourceGroup}
        required={true}
        label="Resource Group Name"
        placeholder="my-resource-group"
        maxWidth={400}
        disabled={disabled}
        onChange={e =>
          onChange({
            ...managedIdentity,
            resourceGroup: e.target.value,
          })
        }
      />
    </>
  );
}
