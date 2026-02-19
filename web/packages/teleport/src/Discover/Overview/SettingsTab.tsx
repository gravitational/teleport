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
import { InfoGuideContainer } from 'shared/components/SlidingSidePanel/InfoGuide';
import Validation from 'shared/components/Validation';

import { SlidingSidePanel } from 'teleport/components/SlidingSidePanel/SlidingSidePanel';
import { DeploymentMethodSection } from 'teleport/Integrations/Enroll/Cloud/Aws/DeploymentMethodSection';
import {
  ConfigurationScopeSection,
  Divider,
  IntegrationSection,
} from 'teleport/Integrations/Enroll/Cloud/Aws/EnrollAws';
import {
  InfoGuideContent,
  InfoGuideTab,
  InfoGuideTitle,
  TerraformInfoGuide,
} from 'teleport/Integrations/Enroll/Cloud/Aws/InfoGuide';
import { RegionsSection } from 'teleport/Integrations/Enroll/Cloud/Aws/RegionsSection';
import { ResourcesSection } from 'teleport/Integrations/Enroll/Cloud/Aws/ResourcesSection';
import { buildTerraformConfig } from 'teleport/Integrations/Enroll/Cloud/Aws/tf_module';
import {
  Ec2Config,
  WildcardRegion,
} from 'teleport/Integrations/Enroll/Cloud/Aws/types';
import { AwsResource } from 'teleport/Integrations/status/AwsOidc/Cards/StatCard';
import { zIndexMap } from 'teleport/Navigation/zIndexMap';
import {
  IntegrationDiscoveryRule,
  integrationService,
  IntegrationWithSummary,
  Regions,
} from 'teleport/services/integrations';
import { useClusterVersion } from 'teleport/useClusterVersion';

import { DeleteIntegrationSection } from './DeleteIntegrationSection';

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

  const { data: ec2Rules, isLoading } = useQuery({
    queryKey: ['integrationRules', stats.name, AwsResource.ec2],
    queryFn: () =>
      integrationService.fetchIntegrationRules(
        integrationName,
        AwsResource.ec2
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

  const getEc2ConfigFromRules = (
    rules?: IntegrationDiscoveryRule[]
  ): Ec2Config => ({
    enabled: rules !== undefined && rules.length > 0,
    tags:
      rules && rules.length > 0
        ? rules[0].labelMatcher.map(l => ({
            name: l.name,
            value: l.value,
          }))
        : [],
  });

  const [updatedRegions, setRegions] = useState<
    WildcardRegion | Regions[] | null
  >(null);
  const [updatedEc2Config, setEc2Config] = useState<Ec2Config | null>(null);

  const regions = updatedRegions ?? getRegionsFromRules(ec2Rules?.rules);
  const ec2Config = updatedEc2Config ?? getEc2ConfigFromRules(ec2Rules?.rules);

  const terraformConfig = useMemo(
    () =>
      buildTerraformConfig({
        integrationName,
        regions: regions,
        ec2Config: ec2Config,
        version: clusterVersion,
      }),
    [integrationName, regions, ec2Config, clusterVersion]
  );

  const isPanelOpen = activeInfoGuideTab !== null;

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
              <Box>
                <ConfigurationScopeSection />
              </Box>
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
                integrationExists={true}
                showVerificationStep={false}
              />
            </Card>

            <DeleteIntegrationSection integrationName={integrationName} />
          </Box>

          <SlidingSidePanel
            isVisible={isPanelOpen}
            skipAnimation={false}
            panelWidth={SETTINGS_PANEL_WIDTH}
            zIndex={zIndexMap.infoGuideSidePanel}
            slideFrom="right"
          >
            <InfoGuideContainer
              onClose={() => onInfoGuideTabChange(null)}
              title={
                <InfoGuideTitle
                  activeSection={activeInfoGuideTab}
                  onSectionChange={onInfoGuideTabChange}
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
        </Flex>
      )}
    </Validation>
  );
}
