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
import { type ReactNode, useState } from 'react';

import {
  Alert,
  Box,
  ButtonIcon,
  ButtonSecondary,
  Card,
  Flex,
  Indicator,
  Mark,
  Text,
} from 'design';
import { Danger } from 'design/Alert';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/DialogConfirmation';
import { Cross } from 'design/Icon';
import { copyToClipboard } from 'design/utils/copyToClipboard';
import Validation from 'shared/components/Validation';

import {
  ResizablePanelLayout,
  ResizableSidePanelMainContent,
} from 'teleport/components/SlidingSidePanel/ResizableSidePanel';
import { DeploymentMethodSection } from 'teleport/Integrations/Enroll/Cloud/Aws/DeploymentMethodSection';
import {
  ScopeSection,
  IntegrationSection,
} from 'teleport/Integrations/Enroll/Cloud/Aws/EnrollAws';
import { InfoGuideContent } from 'teleport/Integrations/Enroll/Cloud/Aws/InfoGuide';
import { ResourcesSection } from 'teleport/Integrations/Enroll/Cloud/Aws/ResourceSection/ResourcesSection';
import { buildTerraformConfig } from 'teleport/Integrations/Enroll/Cloud/Aws/tf_module';
import {
  AwsOrganizationalUnits,
  AwsScope,
  buildMatchers,
  ServiceConfig,
  ServiceConfigs,
  ServiceType,
} from 'teleport/Integrations/Enroll/Cloud/Aws/types';
import { Divider } from 'teleport/Integrations/Enroll/Cloud/Shared';
import { ResizableTerraformInfoGuideSidePanel } from 'teleport/Integrations/Enroll/Cloud/Shared/InfoGuide';
import { AwsResource } from 'teleport/Integrations/status/AwsOidc/Cards/StatCard';
import {
  IntegrationAwsOidc,
  IntegrationDiscoveryRule,
  IntegrationKind,
  integrationService,
  IntegrationWithSummary,
  Regions,
} from 'teleport/services/integrations';
import { useClusterVersion } from 'teleport/useClusterVersion';

import { DeleteIntegrationSection } from '../DeleteIntegrationSection';
import { SettingsTabContainer } from '../SettingsTabContainer';

const organizationFromIntegration = (
  integration?: IntegrationAwsOidc
): AwsOrganizationalUnits | null => {
  const org = integration?.spec?.organization;
  if (org && org.includeUnits?.length > 0) {
    return {
      include: org.includeUnits,
      exclude: org.excludeUnits || [],
    };
  }
  return null;
};

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
  header,
}: {
  stats: IntegrationWithSummary;
  header: ReactNode;
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
      integrationService.fetchIntegration<IntegrationAwsOidc>(integrationName),
  });

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

  const {
    data: rdsRules,
    isLoading: isLoadingRds,
    isError: isRdsError,
  } = useQuery({
    queryKey: ['integrationRules', stats.name, AwsResource.rds],
    queryFn: () =>
      integrationService.fetchIntegrationRules(
        integrationName,
        AwsResource.rds
      ),
  });

  const [updatedConfigs, setUpdatedConfigs] = useState<Partial<ServiceConfigs>>(
    {}
  );
  const [updatedScope, setScope] = useState<AwsScope | null>(null);
  const [updatedOrgUnits, setOrgUnits] =
    useState<AwsOrganizationalUnits | null>(null);

  const [showCopyWarning, setShowCopyWarning] = useState(false);

  if (isLoadingEc2 || isLoadingEks || isLoadingRds || isIntegrationLoading) {
    return (
      <SettingsTabContainer>
        {header}
        <Flex justifyContent="center" mt={6}>
          <Indicator />
        </Flex>
      </SettingsTabContainer>
    );
  }

  if (isEc2Error || isEksError || isRdsError || isIntegrationError) {
    return (
      <SettingsTabContainer>
        {header}
        <Danger>Failed to load the integration settings.</Danger>
      </SettingsTabContainer>
    );
  }

  const initialOrgUnits = organizationFromIntegration(integration);
  const initialScope: AwsScope = initialOrgUnits ? 'organization' : 'account';
  const scope = updatedScope ?? initialScope;
  const isOrganization = scope === 'organization';
  const orgUnits = updatedOrgUnits ??
    initialOrgUnits ?? { include: ['*'], exclude: [] };

  const rdsEnabled = rdsRules.rules.length > 0;

  const configs: ServiceConfigs = {
    ec2: updatedConfigs.ec2 ?? configFromRules(ec2Rules.rules),
    eks: updatedConfigs.eks ?? configFromRules(eksRules.rules),
    rds: updatedConfigs.rds ?? {
      ...configFromRules(rdsRules.rules),
      databaseService: {
        deploy: !rdsEnabled,
        // Note: these fields are not persisted by backend.
        agents: rdsEnabled
          ? []
          : [{ region: '', vpcId: '', securityGroupIds: [] }],
      },
    },
  };

  const updateConfig = (type: ServiceType, patch: Partial<ServiceConfig>) => {
    setUpdatedConfigs(prev => ({
      ...prev,
      [type]: { ...configs[type], ...patch },
    }));
  };

  // TF module currently supports EC2 only
  const supportedConfigs: ServiceConfigs = isOrganization
    ? {
        ...configs,
        eks: { ...configs.eks, enabled: false },
        rds: { ...configs.rds, enabled: false },
      }
    : configs;

  const terraformConfig = buildTerraformConfig({
    integrationName,
    matchers: buildMatchers(supportedConfigs),
    version: clusterVersion,
    orgUnits: isOrganization ? orgUnits : null,
    databaseService: supportedConfigs.rds.enabled
      ? supportedConfigs.rds.databaseService
      : undefined,
  });

  const shouldWarnOnCopy =
    supportedConfigs.rds.enabled &&
    supportedConfigs.rds.databaseService?.deploy === false;

  function copyTerraformConfig() {
    copyToClipboard(terraformConfig);

    if (shouldWarnOnCopy) {
      setShowCopyWarning(true);
    }
  }

  return (
    <Validation>
      {({ validator }) => (
        <ResizablePanelLayout $withHorizontalScroll>
          <ResizableSidePanelMainContent $minWidth={650}>
            <SettingsTabContainer>
              {header}
              <Card p={4} mb={3}>
                <Box mb={4}>
                  <IntegrationSection
                    integrationName={integrationName}
                    onChange={() => {}}
                    disabled={true}
                  />
                </Box>
                <Divider />
                <ScopeSection
                  scope={scope}
                  onScopeChange={setScope}
                  orgUnits={orgUnits}
                  onOrgUnitsChange={setOrgUnits}
                />
                <Divider />
                <ResourcesSection
                  configs={supportedConfigs}
                  onConfigChange={updateConfig}
                  isOrganization={isOrganization}
                />
                <Divider />
                <DeploymentMethodSection
                  isOrganization={isOrganization}
                  terraformConfig={terraformConfig}
                  handleCopy={() => {
                    if (validator.validate() && terraformConfig) {
                      copyTerraformConfig();
                    }
                  }}
                  integrationExists={true}
                  showVerificationStep={false}
                />
              </Card>

              <DeleteIntegrationSection
                integrationName={integrationName}
                kind={IntegrationKind.AwsOidc}
              />
            </SettingsTabContainer>
          </ResizableSidePanelMainContent>

          <ResizableTerraformInfoGuideSidePanel
            infoGuideContent={<InfoGuideContent />}
            terraformConfig={terraformConfig}
            onEditorCopy={
              shouldWarnOnCopy ? () => setShowCopyWarning(true) : undefined
            }
            handleCopy={() => {
              if (validator.validate() && terraformConfig) {
                copyTerraformConfig();
              }
            }}
          />
          {showCopyWarning && (
            <PreserveDbAgentWarning onClose={() => setShowCopyWarning(false)} />
          )}
        </ResizablePanelLayout>
      )}
    </Validation>
  );
}

function PreserveDbAgentWarning({ onClose }: { onClose(): void }) {
  return (
    <Dialog onClose={onClose} open={true}>
      <DialogHeader justifyContent="space-between">
        <DialogTitle>Terraform Module Copied</DialogTitle>
        <ButtonIcon onClick={onClose}>
          <Cross size="small" />
        </ButtonIcon>
      </DialogHeader>
      <DialogContent width="500px">
        <Alert kind="warning" mb={3}>
          Preserve your existing Database Service configuration.
        </Alert>
        <Text typography="body3">
          Keep any existing <Mark>module &quot;db_agent&quot;</Mark> block.
          Removing it from the same Terraform state will cause Terraform to plan
          deleting the Database Service infrastructure.
        </Text>
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={onClose}>Got It</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
