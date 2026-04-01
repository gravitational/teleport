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
import { useMemo, useState } from 'react';

import { Box, Card, Flex, Indicator } from 'design';
import { Info as InfoAlert } from 'design/Alert';
import { copyToClipboard } from 'design/utils/copyToClipboard';
import Validation from 'shared/components/Validation';

import { DeploymentMethodSection } from 'teleport/Integrations/Enroll/Cloud/Azure/DeploymentMethodSection';
import {
  ConfigurationScopeSection,
  IntegrationSection,
} from 'teleport/Integrations/Enroll/Cloud/Azure/EnrollAzure';
import { InfoGuideContent } from 'teleport/Integrations/Enroll/Cloud/Azure/InfoGuide';
import { RegionsSection } from 'teleport/Integrations/Enroll/Cloud/Azure/RegionsSection';
import { ResourcesSection } from 'teleport/Integrations/Enroll/Cloud/Azure/ResourcesSection';
import { buildTerraformConfig } from 'teleport/Integrations/Enroll/Cloud/Azure/tf_module';
import {
  AzureScope,
  VmConfig,
} from 'teleport/Integrations/Enroll/Cloud/Azure/types';
import { Divider } from 'teleport/Integrations/Enroll/Cloud/Shared';
import { RegionOrWildcard } from 'teleport/Integrations/Enroll/Cloud/Shared';
import {
  InfoGuideTab,
  TerraformInfoGuide,
  TerraformInfoGuideSidePanel,
} from 'teleport/Integrations/Enroll/Cloud/Shared/InfoGuide';
import {
  IntegrationDiscoveryRule,
  IntegrationWithSummary,
  integrationService,
  AzureRegion,
  AzureResource,
} from 'teleport/services/integrations';
import { useClusterVersion } from 'teleport/useClusterVersion';

import { DeleteIntegrationSection } from '../DeleteIntegrationSection';

export const SETTINGS_PANEL_WIDTH = 500;

export function SettingsTab({
  stats,
  activeInfoGuideTab,
  onInfoGuideTabChange,
}: {
  stats: IntegrationWithSummary;
  activeInfoGuideTab: InfoGuideTab | null;
  onInfoGuideTabChange: (tab: InfoGuideTab) => void;
}) {
  const integrationName = stats.name;
  const { clusterVersion } = useClusterVersion();

  const { data: vmRules, isLoading } = useQuery({
    queryKey: ['integrationRules', stats.name, 'vm'],
    queryFn: () =>
      integrationService.fetchIntegrationRules(
        integrationName,
        AzureResource.vm
      ),
    enabled: true,
  });

  const getRegionsFromRules = (
    rules?: IntegrationDiscoveryRule[]
  ): RegionOrWildcard<AzureRegion>[] => {
    if (!rules || rules.length === 0) {
      return ['*'];
    }
    const regions = rules.map(rule => rule.region);

    if (regions.includes('*')) {
      return ['*'];
    }

    return regions as RegionOrWildcard<AzureRegion>[];
  };

  const getVmConfigFromRules = (
    rules?: IntegrationDiscoveryRule[]
  ): VmConfig => ({
    enabled: rules !== undefined && rules.length > 0,
    tags:
      rules && rules.length > 0
        ? rules[0].labelMatcher.map(l => ({
            name: l.name,
            value: l.value,
          }))
        : [],
  });

  const getDefaultConfigScope = (): AzureScope => ({
    resource_group: '',
    managed_identity_region: 'eastus',
  });

  const [updatedRegions, setRegions] = useState<
    RegionOrWildcard<AzureRegion>[] | null
  >(null);
  const [updatedVmConfig, setVmConfig] = useState<VmConfig | null>(null);
  const [updatedConfigScope, setConfigScope] = useState<AzureScope | null>(
    null
  );

  const regions = updatedRegions ?? getRegionsFromRules(vmRules?.rules);
  const vmConfig = updatedVmConfig ?? getVmConfigFromRules(vmRules?.rules);
  const configScope = updatedConfigScope ?? getDefaultConfigScope();

  const terraformConfig = useMemo(
    () =>
      buildTerraformConfig({
        integrationName,
        regions: regions,
        vmConfig: vmConfig,
        configScope: configScope,
        version: clusterVersion,
      }),
    [integrationName, regions, vmConfig, configScope, clusterVersion]
  );

  if (isLoading) {
    return (
      <Flex justifyContent="center" mt={6}>
        <Indicator />
      </Flex>
    );
  }

  return (
    <Validation>
      {({ validator }) => (
        <Flex>
          <Box flex="1">
            <Card p={4} mb={3}>
              <InfoAlert
                mb={3}
                details="Review the prerequisites and setup requirements before configuring this integration."
                primaryAction={{
                  content: 'View Info Guide',
                  onClick: () => onInfoGuideTabChange('info'),
                }}
              >
                Before You Begin
              </InfoAlert>
              <Box mb={4}>
                <IntegrationSection
                  integrationName={integrationName}
                  onChange={() => {}}
                  disabled={true}
                />
              </Box>
              <Divider />
              <Box>
                <ConfigurationScopeSection
                  configScope={configScope}
                  onChange={setConfigScope}
                  disabled={false}
                />
              </Box>
              <Divider />
              <ResourcesSection vmConfig={vmConfig} onVmChange={setVmConfig} />
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
                integrationExists={true}
                showVerificationStep={false}
              />
            </Card>

            <DeleteIntegrationSection integrationName={integrationName} />
          </Box>

          <TerraformInfoGuideSidePanel
            panelWidth={SETTINGS_PANEL_WIDTH}
            activeTab={activeInfoGuideTab}
            onTabChange={onInfoGuideTabChange}
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
        </Flex>
      )}
    </Validation>
  );
}
