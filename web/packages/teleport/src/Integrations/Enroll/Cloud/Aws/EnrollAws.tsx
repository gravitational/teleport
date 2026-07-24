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
import { RadioGroup } from 'design/RadioGroup';
import FieldInput from 'shared/components/FieldInput';
import { FieldMultiInput } from 'shared/components/FieldMultiInput/FieldMultiInput';
import Validation from 'shared/components/Validation';
import { requiredIntegrationName } from 'shared/components/Validation/rules';

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
  ContentWithSidePanel,
  InfoGuideSwitch,
  TerraformInfoGuide,
  TerraformInfoGuideSidePanel,
  useTerraformInfoGuide,
} from '../Shared/InfoGuide';
import { DeploymentMethodSection } from './DeploymentMethodSection';
import { InfoGuideContent } from './InfoGuide';
import { ResourcesSection } from './ResourcesSection';
import { buildTerraformConfig } from './tf_module';
import {
  AwsOrganizationalUnits,
  AwsScope,
  buildMatchers,
  ServiceConfig,
  ServiceConfigs,
  ServiceType,
} from './types';

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

  const [scope, setScope] = useState<AwsScope>('account');
  const [orgDiscoveryConfig, setOrgDiscoveryConfig] =
    useState<AwsOrganizationalUnits>({
      include: ['*'],
      exclude: [],
    });

  const [configs, setConfigs] = useState<ServiceConfigs>({
    ec2: { enabled: true, regions: [], tags: [] },
    eks: { enabled: false, regions: [], tags: [], kubeAppDiscovery: true },
  });

  const updateConfig = (type: ServiceType, patch: Partial<ServiceConfig>) => {
    setConfigs(prev => ({
      ...prev,
      [type]: { ...prev[type], ...patch },
    }));
  };

  const terraformConfig = useMemo(
    () =>
      buildTerraformConfig({
        integrationName,
        matchers: buildMatchers(configs),
        version: clusterVersion,
        orgDiscovery: scope === 'organization' ? orgDiscoveryConfig : null,
      }),
    [integrationName, configs, clusterVersion, scope, orgDiscoveryConfig]
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
          <ContentWithSidePanel isPanelOpen={isPanelOpen}>
            <Flex justifyContent="space-between" alignItems="center" mb={1}>
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
            <Container flexDirection="column" p={4} mb={3}>
              <IntegrationSection
                integrationName={integrationName}
                onChange={setIntegrationName}
                disabled={isFetching}
              />
              <Divider />
              <IamSection
                scope={scope}
                onScopeChange={setScope}
                orgDiscoveryConfig={orgDiscoveryConfig}
                onOrgDiscoveryChange={setOrgDiscoveryConfig}
              />
              <Divider />
              <ResourcesSection
                configs={configs}
                onConfigChange={updateConfig}
              />
              <Divider />
              <DeploymentMethodSection
                isOrganization={scope === 'organization'}
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

type IamSectionProps = {
  scope: AwsScope;
  onScopeChange: (scope: AwsScope) => void;
  orgDiscoveryConfig: AwsOrganizationalUnits;
  onOrgDiscoveryChange: (config: AwsOrganizationalUnits) => void;
};

export function IamSection({
  scope,
  onScopeChange,
  orgDiscoveryConfig,
  onOrgDiscoveryChange,
}: IamSectionProps) {
  const isOrganization = scope === 'organization';

  return (
    <>
      <Flex alignItems="center" fontSize={4} fontWeight="medium" mb={1}>
        <CircleNumber>2</CircleNumber>
        IAM Role
      </Flex>
      <Text ml={4} mb={3}>
        Discover resources in a single AWS account or across multiple accounts
        using AWS Organizations.
      </Text>
      <Box ml={4} mb={3}>
        <RadioGroup
          name="awsScope"
          options={[
            { value: 'account', label: 'Single Account' },
            { value: 'organization', label: 'Organization' },
          ]}
          value={scope}
          size="small"
          onChange={value => onScopeChange(value as AwsScope)}
        />
      </Box>
      {isOrganization && (
        <>
          <Box ml={4} mb={3} maxWidth={432}>
            <FieldMultiInput
              label="Include Organizational Units"
              value={orgDiscoveryConfig.include}
              placeholder="ou-xy-abcdefgh, r-xy, or *"
              onChange={include =>
                onOrgDiscoveryChange({ ...orgDiscoveryConfig, include })
              }
            />
          </Box>
          <Box ml={4} mb={3} maxWidth={432}>
            <FieldMultiInput
              label="Exclude Organizational Units"
              value={orgDiscoveryConfig.exclude}
              placeholder="ou-xy-abcdefgh or r-xy"
              onChange={exclude =>
                onOrgDiscoveryChange({ ...orgDiscoveryConfig, exclude })
              }
            />
          </Box>
        </>
      )}
    </>
  );
}
