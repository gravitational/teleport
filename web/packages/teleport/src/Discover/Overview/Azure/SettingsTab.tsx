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
import { Danger } from 'design/Alert';
import { copyToClipboard } from 'design/utils/copyToClipboard';
import Validation from 'shared/components/Validation';

import { ApplyTerraformSection } from 'teleport/Integrations/Enroll/Cloud/Azure/ApplyTerraformSection';
import {
  ManagedIdentitySection,
  IntegrationSection,
} from 'teleport/Integrations/Enroll/Cloud/Azure/EnrollAzure';
import { InfoGuideContent } from 'teleport/Integrations/Enroll/Cloud/Azure/InfoGuide';
import { ResourcesSection } from 'teleport/Integrations/Enroll/Cloud/Azure/ResourcesSection';
import { buildTerraformConfig } from 'teleport/Integrations/Enroll/Cloud/Azure/tf_module';
import {
  AzureManagedIdentity,
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
  IntegrationAzureOidc,
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

  const {
    data: vmRules,
    isLoading: isRulesLoading,
    isError: isRulesError,
  } = useQuery({
    queryKey: ['integrationRules', stats.name, 'vm'],
    queryFn: () =>
      integrationService.fetchIntegrationRules(
        integrationName,
        AzureResource.vm
      ),
  });

  const { data: integration, isLoading: isIntegrationLoading } = useQuery({
    queryKey: ['integration', integrationName],
    queryFn: () =>
      integrationService.fetchIntegration<IntegrationAzureOidc>(
        integrationName
      ),
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

  const getSubscriptionsFromRules = (rules?: IntegrationDiscoveryRule[]) => [
    ...new Set((rules || []).flatMap(r => r.subscriptions || [])),
  ];

  const getResourceGroupsFromRules = (rules?: IntegrationDiscoveryRule[]) => [
    ...new Set((rules || []).flatMap(r => r.resourceGroups || [])),
  ];

  const getVmConfigFromRules = (
    rules?: IntegrationDiscoveryRule[]
  ): VmConfig => ({
    type: 'vm',
    enabled: rules !== undefined && rules.length > 0,
    regions: getRegionsFromRules(rules),
    subscriptions: getSubscriptionsFromRules(rules),
    resourceGroups: getResourceGroupsFromRules(rules),
    tags:
      rules && rules.length > 0
        ? rules[0].labelMatcher.map(l => ({
            name: l.name,
            value: l.value,
          }))
        : [],
  });

  const getManagedIdentityFromIntegration = (
    integration?: IntegrationAzureOidc
  ): AzureManagedIdentity => ({
    resourceGroup: integration?.spec?.managedIdentity?.resourceGroup || '',
    region: (integration?.spec?.managedIdentity?.region ||
      'eastus') as AzureRegion,
  });

  const [updatedRegions] = useState<RegionOrWildcard<AzureRegion>[] | null>(
    null
  );
  const [updatedVmConfig, setVmConfig] = useState<VmConfig | null>(null);
  const [updatedManagedIdentity, setManagedIdentity] =
    useState<AzureManagedIdentity | null>(null);

  const regions = updatedRegions ?? getRegionsFromRules(vmRules?.rules);
  const vmConfig = updatedVmConfig ?? getVmConfigFromRules(vmRules?.rules);
  const managedIdentity =
    updatedManagedIdentity ?? getManagedIdentityFromIntegration(integration);

  const terraformConfig = useMemo(
    () =>
      buildTerraformConfig({
        integrationName,
        vmConfig: { ...vmConfig, regions },
        managedIdentity: managedIdentity,
        version: clusterVersion,
      }),
    [integrationName, regions, vmConfig, managedIdentity, clusterVersion]
  );

  const isLoading = isRulesLoading || isIntegrationLoading;
  if (isLoading) {
    return (
      <Flex justifyContent="center" mt={6}>
        <Indicator />
      </Flex>
    );
  }

  if (isRulesError) {
    return <Danger>Failed to load the integration settings.</Danger>;
  }

  return (
    <Validation>
      {({ validator }) => (
        <Flex>
          <Box flex="1">
            <Card p={4} mb={3}>
              <Box mb={4}>
                <IntegrationSection
                  integrationName={integrationName}
                  onChange={() => {}}
                  disabled={true}
                />
              </Box>
              <Divider />
              <Box>
                <ManagedIdentitySection
                  managedIdentity={managedIdentity}
                  onChange={setManagedIdentity}
                  disabled={false}
                />
              </Box>
              <Divider />
              <ResourcesSection vmConfig={vmConfig} onVmChange={setVmConfig} />
              <Divider />
              <ApplyTerraformSection
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
