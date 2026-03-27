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
import { useState } from 'react';

import { Box, Card, Flex, Indicator } from 'design';
import { Info as InfoAlert } from 'design/Alert';
import { copyToClipboard } from 'design/utils/copyToClipboard';
import Validation from 'shared/components/Validation';

import { DeploymentMethodSection } from 'teleport/Integrations/Enroll/Cloud/Aws/DeploymentMethodSection';
import { IntegrationSection } from 'teleport/Integrations/Enroll/Cloud/Aws/EnrollAws';
import { InfoGuideContent } from 'teleport/Integrations/Enroll/Cloud/Aws/InfoGuide';
import { ResourcesSection } from 'teleport/Integrations/Enroll/Cloud/Aws/ResourcesSection';
import { buildTerraformConfig } from 'teleport/Integrations/Enroll/Cloud/Aws/tf_module';
import {
  ServiceConfig,
  ServiceConfigs,
  ServiceType,
} from 'teleport/Integrations/Enroll/Cloud/Aws/types';
import {
  Divider,
  WildcardRegion,
} from 'teleport/Integrations/Enroll/Cloud/Shared';
import {
  InfoGuideTab,
  TerraformInfoGuide,
  TerraformInfoGuideSidePanel,
} from 'teleport/Integrations/Enroll/Cloud/Shared/InfoGuide';
import { AwsResource } from 'teleport/Integrations/status/AwsOidc/Cards/StatCard';
import {
  IntegrationDiscoveryRule,
  integrationService,
  IntegrationWithSummary,
  Regions,
} from 'teleport/services/integrations';
import { useClusterVersion } from 'teleport/useClusterVersion';

import { DeleteIntegrationSection } from '../DeleteIntegrationSection';
import { SETTINGS_PANEL_WIDTH } from '../SettingsTab';

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

  const { data: ec2Rules, isLoading: isLoadingEc2 } = useQuery({
    queryKey: ['integrationRules', stats.name, AwsResource.ec2],
    queryFn: () =>
      integrationService.fetchIntegrationRules(
        integrationName,
        AwsResource.ec2
      ),
    enabled: true,
  });

  const { data: eksRules, isLoading: isLoadingEks } = useQuery({
    queryKey: ['integrationRules', stats.name, AwsResource.eks],
    queryFn: () =>
      integrationService.fetchIntegrationRules(
        integrationName,
        AwsResource.eks
      ),
    enabled: true,
  });

  const getRegionsFromRules = (
    rules?: IntegrationDiscoveryRule[]
  ): WildcardRegion | Regions[] => {
    if (!rules || rules.length === 0) {
      return ['*'] as WildcardRegion;
    }
    const regions = rules.map(rule => rule.region);

    if (regions.includes('*') || regions.includes('aws-global')) {
      return ['*'] as WildcardRegion;
    }

    return regions.length === 0
      ? (['*'] as WildcardRegion)
      : (regions as Regions[]);
  };

  const getConfigFromRules = (
    rules?: IntegrationDiscoveryRule[]
  ): ServiceConfig => ({
    enabled: rules !== undefined && rules.length > 0,
    regions: getRegionsFromRules(rules),
    tags:
      rules && rules.length > 0
        ? rules[0].labelMatcher.map(l => ({
            name: l.name,
            value: l.value,
          }))
        : [],
  });

  const [updatedConfigs, setUpdatedConfigs] = useState<Partial<ServiceConfigs>>(
    {}
  );

  const configs: ServiceConfigs = {
    ec2: updatedConfigs.ec2 ?? getConfigFromRules(ec2Rules?.rules),
    eks: updatedConfigs.eks ?? getConfigFromRules(eksRules?.rules),
  };

  const updateConfig = (type: ServiceType, config: ServiceConfig) => {
    setUpdatedConfigs(prev => ({ ...prev, [type]: config }));
  };

  const terraformConfig = buildTerraformConfig({
    integrationName,
    configs,
    version: clusterVersion,
  });

  if (isLoadingEc2 || isLoadingEks) {
    return (
      <Flex justifyContent="center" mt={6}>
        <Indicator />
      </Flex>
    );
  }

  return (
    <Validation>
      {({ validator }) => (
        <Flex pt={3}>
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
              <ResourcesSection
                configs={configs}
                onConfigChange={updateConfig}
              />
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
