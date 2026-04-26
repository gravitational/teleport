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
import { Danger } from 'design/Alert';
import { copyToClipboard } from 'design/utils/copyToClipboard';
import Validation from 'shared/components/Validation';

import { DeploymentMethodSection } from 'teleport/Integrations/Enroll/Cloud/Aws/DeploymentMethodSection';
import { IntegrationSection } from 'teleport/Integrations/Enroll/Cloud/Aws/EnrollAws';
import { InfoGuideContent } from 'teleport/Integrations/Enroll/Cloud/Aws/InfoGuide';
import { ResourcesSection } from 'teleport/Integrations/Enroll/Cloud/Aws/ResourcesSection';
import { buildTerraformConfig } from 'teleport/Integrations/Enroll/Cloud/Aws/tf_module';
import {
  buildMatchers,
  ServiceConfig,
  ServiceConfigs,
  ServiceType,
} from 'teleport/Integrations/Enroll/Cloud/Aws/types';
import { Divider } from 'teleport/Integrations/Enroll/Cloud/Shared';
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

const configFromRules = (rules: IntegrationDiscoveryRule[]): ServiceConfig => {
  const regions = rules.map(r => r.region);
  const isWildcard = regions.includes('*') || regions.includes('aws-global');

  return {
    enabled: rules.length > 0,
    regions: isWildcard || rules.length === 0 ? [] : (regions as Regions[]),
    tags:
      rules.length > 0
        ? rules[0].labelMatcher.map(l => ({
            name: l.name,
            value: l.value,
          }))
        : [],
    kubeAppDiscovery: rules[0]?.kubeAppDiscovery ?? false,
  };
};

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
    data: ec2Rules,
    isLoading: isLoadingEc2,
    isError: isEc2Error,
  } = useQuery({
    queryKey: ['integrationRules', stats.name, AwsResource.ec2],
    queryFn: () =>
      integrationService.fetchIntegrationRules(
        integrationName,
        AwsResource.ec2
      ),
  });

  const {
    data: eksRules,
    isLoading: isLoadingEks,
    isError: isEksError,
  } = useQuery({
    queryKey: ['integrationRules', stats.name, AwsResource.eks],
    queryFn: () =>
      integrationService.fetchIntegrationRules(
        integrationName,
        AwsResource.eks
      ),
  });

  const [updatedConfigs, setUpdatedConfigs] = useState<Partial<ServiceConfigs>>(
    {}
  );

  if (isLoadingEc2 || isLoadingEks) {
    return (
      <Flex justifyContent="center" mt={6}>
        <Indicator />
      </Flex>
    );
  }

  if (isEc2Error || isEksError) {
    return <Danger>Failed to load the integration settings.</Danger>;
  }

  const configs: ServiceConfigs = {
    ec2: updatedConfigs.ec2 ?? configFromRules(ec2Rules.rules),
    eks: updatedConfigs.eks ?? configFromRules(eksRules.rules),
  };

  const updateConfig = (type: ServiceType, patch: Partial<ServiceConfig>) => {
    setUpdatedConfigs(prev => ({
      ...prev,
      [type]: { ...configs[type], ...patch },
    }));
  };

  const terraformConfig = buildTerraformConfig({
    integrationName,
    matchers: buildMatchers(configs),
    version: clusterVersion,
  });

  return (
    <Validation>
      {({ validator }) => (
        <Flex pt={3}>
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
