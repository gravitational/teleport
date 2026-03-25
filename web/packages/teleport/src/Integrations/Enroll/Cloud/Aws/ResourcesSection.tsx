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

import { useState } from 'react';
import styled from 'styled-components';

import { Box, ButtonText, Flex, Text } from 'design';
import { FieldRadio } from 'design/FieldRadio';
import * as Icons from 'design/Icon';
import { IconTooltip } from 'design/Tooltip';
import { FieldCheckbox } from 'shared/components/FieldCheckbox';
import { requiredField } from 'shared/components/Validation/rules';

import {
  LabelsInput,
  type Label,
  type LabelsRule,
} from 'teleport/components/LabelsInput/LabelsInput';
import { Regions as AwsRegion } from 'teleport/services/integrations';

import { RegionMultiSelector } from '../RegionMultiSelector';
import { CircleNumber } from './EnrollAws';
import { awsRegionGroups } from './regions';
import {
  AwsLabel,
  ServiceConfig,
  ServiceConfigs,
  ServiceType,
  WildcardRegion,
} from './types';

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

const isWildcard = (
  regions: WildcardRegion | AwsRegion[]
): regions is WildcardRegion => regions.length === 1 && regions[0] === '*';

const requiredAtLeastOneRegion = (regions: AwsRegion[]) => () => {
  if (!regions || regions.length === 0) {
    return {
      valid: false,
      message: 'At least one region must be selected',
    };
  }
  return { valid: true };
};

type ServiceDescriptor = {
  type: ServiceType;
  label: string;
  resourceName: string;
  description: string;
  allowWildcardRegions: boolean;
};

const serviceDescriptors: ServiceDescriptor[] = [
  {
    type: 'ec2',
    label: 'EC2 Instances',
    resourceName: 'EC2 instances',
    description:
      'Discover EC2 instances and establish SSH access through Teleport.',
    allowWildcardRegions: true,
  },
  {
    type: 'eks',
    label: 'EKS Clusters',
    resourceName: 'EKS clusters',
    description:
      'Discover EKS clusters and configure Kubernetes access through Teleport.',
    allowWildcardRegions: false,
  },
];

type ResourcesSectionProps = {
  configs: ServiceConfigs;
  onConfigChange: (type: ServiceType, config: ServiceConfig) => void;
};

export function ResourcesSection({
  configs,
  onConfigChange,
}: ResourcesSectionProps) {
  return (
    <>
      <Flex alignItems="center" fontSize={4} fontWeight="medium" mb={1}>
        <CircleNumber>2</CircleNumber>
        Resource Types
      </Flex>
      <Text ml={4} mb={3}>
        Select which AWS resource types to automatically discover and enroll.
      </Text>
      {serviceDescriptors.map((desc, i) => (
        <Box key={desc.type} mt={i > 0 ? 3 : 0}>
          <AwsService
            label={desc.label}
            helperText={desc.description}
            resourceName={desc.resourceName}
            tooltipText={`Filter for ${desc.resourceName} by their tags. If no tags are added, Teleport will enroll all ${desc.resourceName}.`}
            config={configs[desc.type]}
            onChange={config => onConfigChange(desc.type, config)}
            allowWildcardRegions={desc.allowWildcardRegions}
          />
        </Box>
      ))}
    </>
  );
}

type AwsServiceProps = {
  label: string;
  helperText: React.ReactNode;
  resourceName: string;
  tooltipText: string;
  config: ServiceConfig;
  onChange: (config: ServiceConfig) => void;
  allowWildcardRegions: boolean;
};

function AwsService({
  label,
  helperText,
  resourceName,
  tooltipText,
  config,
  onChange,
  allowWildcardRegions,
}: AwsServiceProps) {
  const [showTags, setShowTags] = useState(config.tags.length > 0);

  const toggle = () => {
    onChange({
      ...config,
      enabled: !config.enabled,
    });
  };

  const handleRegionModeChange = (wildcard: boolean) => {
    onChange({
      ...config,
      regions: wildcard ? (['*'] as WildcardRegion) : ([] as AwsRegion[]),
    });
  };

  const handleRegionsChange = (regions: AwsRegion[]) => {
    onChange({ ...config, regions });
  };

  return (
    <>
      <FieldCheckbox
        mb={2}
        size="small"
        label={label}
        helperText={helperText}
        checked={config.enabled}
        onChange={toggle}
      />
      {config.enabled && (
        <Box ml={4}>
          <Box mb={3}>
            <Text fontSize="small" mb={1} fontWeight="medium">
              Regions
            </Text>
            {allowWildcardRegions ? (
              <>
                <FieldRadio
                  name={`${label}-regions`}
                  label={<RadioLabel selected={false}>All regions</RadioLabel>}
                  size="small"
                  checked={isWildcard(config.regions)}
                  onChange={() => handleRegionModeChange(true)}
                  mb={1}
                />
                <FieldRadio
                  name={`${label}-regions`}
                  label={
                    <RadioLabel selected={false}>
                      Select specific regions
                    </RadioLabel>
                  }
                  size="small"
                  checked={!isWildcard(config.regions)}
                  onChange={() => handleRegionModeChange(false)}
                  mb={1}
                />
                {!isWildcard(config.regions) && (
                  <Box mt={2}>
                    <RegionMultiSelector
                      regionGroups={awsRegionGroups}
                      selectedRegions={config.regions}
                      onChange={handleRegionsChange}
                      disabled={false}
                      required={true}
                      rule={requiredAtLeastOneRegion}
                    />
                  </Box>
                )}
              </>
            ) : (
              <Box mt={1}>
                <RegionMultiSelector
                  regionGroups={awsRegionGroups}
                  selectedRegions={
                    isWildcard(config.regions) ? [] : config.regions
                  }
                  onChange={handleRegionsChange}
                  disabled={false}
                  required={true}
                  rule={requiredAtLeastOneRegion}
                />
              </Box>
            )}
          </Box>
          <FilterButton onClick={() => setShowTags(prev => !prev)}>
            <Flex alignItems="center" gap={1} mb={2}>
              <FilterChevron size="small" expanded={showTags} />
              Filter by tag
              <IconTooltip kind="info">
                <Text>{tooltipText}</Text>
              </IconTooltip>
            </Flex>
          </FilterButton>
          {showTags && (
            <Box mb={2}>
              <Box width={400}>
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
                  setLabels={(tags: Label[]) =>
                    onChange({
                      ...config,
                      tags: tags as AwsLabel[],
                    })
                  }
                  rule={nonEmptyTags}
                />
              </Box>
            </Box>
          )}
          {config.tags.length === 0 && (
            <Flex alignItems="center" gap={1} mt={1}>
              <Icons.Warning size="small" color="warning.main" />
              <Text fontSize="small" color="text.slightlyMuted">
                If no tags are specified, all {resourceName} in the selected
                regions will be enrolled.
              </Text>
            </Flex>
          )}
        </Box>
      )}
    </>
  );
}

const FilterButton = styled(ButtonText)`
  background: transparent;
  border: none;
  padding: 0;
  color: ${props => props.theme.colors.text.main};
  cursor: pointer;
  font: inherit;

  &:hover {
    color: ${props => props.theme.colors.text.main};
    background: transparent;
  }

  &:focus-visible {
    outline: 2px solid
      ${props => props.theme.colors.interactive.solid.primary.default};
    outline-offset: 2px;
  }
`;

const FilterChevron = styled(Icons.ChevronRight)<{
  expanded: boolean;
}>`
  transition: transform 0.2s ease-in-out;
  transform: ${props => (props.expanded ? 'rotate(90deg)' : 'none')};
`;

const RadioLabel = styled(Flex)<{ selected: boolean }>`
  font-weight: ${props => (props.selected ? '600' : 'inherit')};
`;
