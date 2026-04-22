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

import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';

import { Box, Card, Flex, Indicator } from 'design';
import { Danger, Info } from 'design/Alert';
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

const vmConfigFromRules = (rules?: IntegrationDiscoveryRule[]): VmConfig => {
  const regions: (AzureRegion | '*')[] =
    !rules?.length || rules.some(r => r.region === '*')
      ? ['*']
      : rules.map(r => r.region as AzureRegion);

  const subscriptions = [
    ...new Set((rules || []).flatMap(r => r.subscriptions || [])),
  ];

  const resourceGroups = [
    ...new Set((rules || []).flatMap(r => r.resourceGroups || [])),
  ].filter(g => g !== '*');

  return {
    type: 'vm',
    enabled: rules !== undefined && rules.length > 0,
    regions,
    subscriptions,
    resourceGroups,
    tags:
      rules && rules.length > 0
        ? rules[0].labelMatcher.map(l => ({ name: l.name, value: l.value }))
        : [],
  };
};

const managedIdentityFromIntegration = (
  integration?: IntegrationAzureOidc
): AzureManagedIdentity => ({
  resourceGroup: integration?.spec?.managedIdentity?.resourceGroup || '',
  region: (integration?.spec?.managedIdentity?.region ||
    'eastus') as AzureRegion,
});

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
    data: integration,
    isLoading: isIntegrationLoading,
    isError: isIntegrationError,
  } = useQuery({
    queryKey: ['integration', integrationName],
    queryFn: () =>
      integrationService.fetchIntegration<IntegrationAzureOidc>(
        integrationName
      ),
  });

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

  const [updatedVmConfig, setVmConfig] = useState<VmConfig | null>(null);
  const [updatedManagedIdentity, setManagedIdentity] =
    useState<AzureManagedIdentity | null>(null);

  if (isRulesLoading || isIntegrationLoading) {
    return (
      <Flex justifyContent="center" mt={6}>
        <Indicator />
      </Flex>
    );
  }

  if (isIntegrationError || isRulesError) {
    return <Danger>Failed to load the integration settings.</Danger>;
  }

  const hasWildcardSubscription =
    vmRules?.rules?.some(r => r.subscriptions?.includes('*')) ?? false;

  // relax uuid validation if wildcard subscription returned in rules
  const allowWildcardSubscriptions = updatedVmConfig
    ? false
    : hasWildcardSubscription;

  const vmConfig = updatedVmConfig ?? vmConfigFromRules(vmRules?.rules);
  const managedIdentity =
    updatedManagedIdentity ?? managedIdentityFromIntegration(integration);

  const terraformConfig = buildTerraformConfig({
    integrationName,
    vmConfig,
    managedIdentity,
    version: clusterVersion,
  });

  return (
    <Validation>
      {({ validator }) => (
        <Flex>
          <Box flex="1">
            {hasWildcardSubscription && (
              <Info mb={3}>
                This integration was configured with a wildcard subscription
                matcher in Terraform, which is currently unsupported by this
                form. Please update your Terraform configuration directly.
              </Info>
            )}
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
              <ResourcesSection
                vmConfig={vmConfig}
                onVmChange={setVmConfig}
                allowWildcardSubscriptions={allowWildcardSubscriptions}
              />
              <Divider />
              <ApplyTerraformSection
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
