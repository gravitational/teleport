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

import { Box, Flex, Text } from 'design';
import { IconTooltip } from 'design/Tooltip';
import { FieldCheckbox } from 'shared/components/FieldCheckbox';
import { Option } from 'shared/components/Select';
import { useRule } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import {
  LabelsInput,
  type Label,
  type LabelsRule,
} from 'teleport/components/LabelsInput/LabelsInput';
import { Regions as AwsRegion } from 'teleport/services/integrations';

import { CircleNumber, TextWeighted, TextWeightedSmall } from '../../Shared';
import { RegionSelect } from '../../Shared/RegionSelect';
import { awsRegionOptionGroups, awsRegionOptions } from '../regions';
import { TagsButton, TagsChevron } from '../Tags';
import {
  AwsLabel,
  DbSvcDeployment,
  ServiceConfig,
  ServiceConfigs,
  ServiceType,
} from '../types';
import { KubeAppDiscoverySection } from './EksSection';
import { DatabaseServiceSection } from './RdsSection';

const requiredResourceType = (configs: ServiceConfigs) => () => {
  const hasEnabledService = Object.values(configs).some(c => c.enabled);

  if (hasEnabledService) {
    return { valid: true };
  }

  return {
    valid: false,
    message: 'Select at least one resource type',
  };
};

const nonEmptyTags: LabelsRule = (labels: Label[]) => () => {
  const results = labels.map(label => ({
    name: requiredField('Please enter a tag key')(label.name)(),
    value: requiredField('Please enter a tag value')(label.value)(),
  }));
  return {
    valid: results.every(r => r.name.valid && r.value.valid),
    results: results,
  };
};

const fieldWidth = 400;
// tags needs additional room for the close X on the right.
const tagsWidth = fieldWidth + 32;

type ResourcesSectionProps = {
  configs: ServiceConfigs;
  onConfigChange: (type: ServiceType, patch: Partial<ServiceConfig>) => void;
  isOrganization?: boolean;
};

export function ResourcesSection({
  configs,
  onConfigChange,
  isOrganization = false,
}: ResourcesSectionProps) {
  const { valid, message } = useRule(requiredResourceType(configs));
  const hasError = !valid;
  const databaseService = configs.rds.databaseService ?? {
    deploy: false,
    agents: [],
  };

  function getDbAgentRowsForRegions(regions: AwsRegion[]) {
    if (!regions.length) {
      return [{ region: '' as const, vpcId: '', securityGroupIds: [] }];
    }

    const agents = databaseService.agents.filter(
      agent => agent.region !== '' && regions.includes(agent.region)
    );

    regions.forEach(region => {
      if (!agents.some(agent => agent.region === region)) {
        agents.push({ region, vpcId: '', securityGroupIds: [] });
      }
    });

    return agents;
  }

  function updateRdsAwsDiscoveryModule(patch: Partial<ServiceConfig>) {
    if (databaseService.deploy && patch.regions !== undefined) {
      onConfigChange('rds', {
        ...patch,
        databaseService: {
          ...databaseService,
          agents: getDbAgentRowsForRegions(patch.regions),
        },
      });
      return;
    }

    onConfigChange('rds', patch);
  }

  function updateDbAgentModule(agents: DbSvcDeployment[]) {
    const regionSet = new Set(
      agents.map(agent => agent.region).filter(region => region !== '')
    );

    onConfigChange('rds', {
      regions: [...regionSet],
      databaseService: { ...databaseService, agents },
    });
  }

  return (
    <>
      <Flex alignItems="center" fontSize={4} fontWeight="medium" mb={1}>
        <CircleNumber>3</CircleNumber>
        Resource Types
      </Flex>
      <Text ml={4} mb={3}>
        Select which AWS resource types to automatically discover and enroll in
        your Teleport cluster.
      </Text>
      {hasError && (
        <Text
          ml={4}
          mb={3}
          typography="body3"
          color="interactive.solid.danger.default"
        >
          {message}
        </Text>
      )}

      <AwsService
        resourceKind="ec2"
        config={configs.ec2}
        onUpdate={patch => onConfigChange('ec2', patch)}
        tagTooltip="Match EC2 instances by their tags. If no tags are added, Teleport will match and enroll all EC2 instances."
      />

      <Box mt={4}>
        <AwsService
          resourceKind="rds"
          config={configs.rds}
          onUpdate={updateRdsAwsDiscoveryModule}
          tagTooltip="Match RDS databases by their tags. If no tags are added, Teleport will match and enroll all RDS databases."
          disabled={isOrganization}
        >
          <DatabaseServiceSection
            isChecked={databaseService.deploy}
            agents={databaseService.agents}
            onAgentsChange={updateDbAgentModule}
            check={() =>
              onConfigChange('rds', {
                databaseService: {
                  deploy: true,
                  agents: getDbAgentRowsForRegions(configs.rds.regions),
                },
              })
            }
            unCheck={() =>
              onConfigChange('rds', {
                databaseService: { ...databaseService, deploy: false },
              })
            }
          />
        </AwsService>
      </Box>

      <Box mt={2}>
        <AwsService
          resourceKind="eks"
          config={configs.eks}
          onUpdate={patch => onConfigChange('eks', patch)}
          tagTooltip="Match EKS clusters by their tags. If no tags are added, Teleport will match and enroll all EKS clusters."
          disabled={isOrganization}
        >
          <KubeAppDiscoverySection
            disabled={isOrganization}
            isChecked={
              configs.eks.enabled &&
              !isOrganization &&
              configs.eks.kubeAppDiscovery
            }
            onUpdate={() =>
              onConfigChange('eks', {
                kubeAppDiscovery: !configs.eks.kubeAppDiscovery,
              })
            }
          />
        </AwsService>
      </Box>
    </>
  );
}

function AwsService({
  config,
  onUpdate,
  tagTooltip,
  disabled = false,
  children,
  resourceKind,
}: {
  config: ServiceConfig;
  onUpdate: (update: Partial<ServiceConfig>) => void;
  tagTooltip: string;
  disabled?: boolean;
  children?: React.ReactNode;
  resourceKind: 'ec2' | 'eks' | 'rds';
}) {
  const [tagsExpanded, setTagsExpanded] = useState(config.tags.length > 0);

  const selectedOptions: Option<AwsRegion>[] = useMemo(
    () => awsRegionOptions.filter(opt => config.regions.includes(opt.value)),
    [config.regions]
  );

  let pluralDesc = '';
  let singularDesc = '';
  let label = '';
  switch (resourceKind) {
    case 'ec2':
      pluralDesc = 'EC2 instances';
      singularDesc = 'An instance';
      label = 'EC2 Instances';
      break;
    case 'eks':
      pluralDesc = 'EKS clusters';
      singularDesc = 'A cluster';
      label = 'EKS Clusters';
      break;
    case 'rds':
      pluralDesc = 'RDS clusters';
      singularDesc = 'A database';
      label = 'RDS Databases';
      break;
    default:
      resourceKind satisfies never;
  }

  return (
    <Box ml={4}>
      <FieldCheckbox
        mb={2}
        size="large"
        label={label}
        labelFontWeight="regular"
        checked={config.enabled && !disabled}
        disabled={disabled}
        onChange={() => onUpdate({ enabled: !config.enabled })}
      />
      {disabled && (
        <Text ml={5} mt={-2} color="text.slightlyMuted" fontWeight={'regular'}>
          Not yet supported for organization wide discovery - select “Single
          account” scope to enable.
        </Text>
      )}
      {config.enabled && !disabled && (
        <Box>
          <Box ml={5} mb={2}>
            <TextWeighted>Discovery filters</TextWeighted>
            <TextWeightedSmall>
              Narrow which {pluralDesc} get discovered by region and tags.{' '}
              {singularDesc} must match both to be included.
            </TextWeightedSmall>
          </Box>
          <Box ml={7}>
            <Box mb={-2} width={fieldWidth}>
              <TextWeighted>Region</TextWeighted>
              <RegionSelect
                ariaLabel={`${label} regions`}
                isMulti={true}
                options={awsRegionOptionGroups}
                value={selectedOptions}
                onChange={options =>
                  onUpdate({
                    regions: options.map(opt => opt.value),
                  })
                }
                placeholder="All regions"
              />
            </Box>
            <TagsButton onClick={() => setTagsExpanded(prev => !prev)}>
              <Flex alignItems="center" gap={1} mb={2}>
                <TagsChevron size="medium" expanded={tagsExpanded} />
                <TextWeighted>Match by tag</TextWeighted>
                <IconTooltip kind="info">
                  <Text>{tagTooltip}</Text>
                </IconTooltip>
              </Flex>
            </TagsButton>
            <TextWeightedSmall ml={4} mt={-2}>
              Leave empty to discover all {pluralDesc} in the selected region(s)
            </TextWeightedSmall>
            {tagsExpanded && (
              <Box mb={2} ml={4} mt={2}>
                <Box width={tagsWidth}>
                  <LabelsInput
                    adjective="tag"
                    labels={config.tags as Label[]}
                    labelKey={{
                      fieldName: 'Key',
                      placeholder: 'Environment',
                    }}
                    labelVal={{
                      fieldName: 'Value',
                      placeholder: 'production',
                    }}
                    setLabels={t => onUpdate({ tags: t as AwsLabel[] })}
                    rule={nonEmptyTags}
                  />
                </Box>
              </Box>
            )}
            {children}
          </Box>
        </Box>
      )}
    </Box>
  );
}
