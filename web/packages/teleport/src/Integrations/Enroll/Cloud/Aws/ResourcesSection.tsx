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
import * as Icons from 'design/Icon';
import { IconTooltip } from 'design/Tooltip';
import { FieldCheckbox } from 'shared/components/FieldCheckbox';
import { useRule } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import {
  LabelsInput,
  type Label,
  type LabelsRule,
} from 'teleport/components/LabelsInput/LabelsInput';

import { RegionMultiSelector } from '../RegionMultiSelector';
import { CircleNumber } from '../Shared';
import { CloudRegion } from '../Shared/types';
import { awsRegionGroups } from './regions';
import { AwsLabel, ServiceConfig, ServiceConfigs, ServiceType } from './types';

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

const requiredRegions =
  (allowAll: boolean) => (selection: CloudRegion[]) => () => {
    if (allowAll || selection.length > 0) {
      return { valid: true };
    }
    return {
      valid: false,
      message: 'At least one region must be selected',
    };
  };

type ServiceDescriptor = {
  type: ServiceType;
  label: string;
  resourceName: string;
  allowWildcardRegions: boolean;
};

const serviceDescriptors: ServiceDescriptor[] = [
  {
    type: 'ec2',
    label: 'EC2 Instances',
    resourceName: 'EC2 instances',
    allowWildcardRegions: true,
  },
  {
    type: 'eks',
    label: 'EKS Clusters',
    resourceName: 'EKS clusters',
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
  const { valid, message } = useRule(requiredResourceType(configs));
  const hasError = !valid;

  return (
    <>
      <Flex alignItems="center" fontSize={4} fontWeight="medium" mb={1}>
        <CircleNumber>2</CircleNumber>
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
      {serviceDescriptors.map((desc, i) => (
        <Box key={desc.type} mt={i > 0 ? 3 : 0}>
          <AwsService
            label={desc.label}
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
  tooltipText: string;
  config: ServiceConfig;
  onChange: (config: ServiceConfig) => void;
  allowWildcardRegions: boolean;
};

function AwsService({
  label,
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

  const handleRegionsChange = (regions: CloudRegion[]) => {
    onChange({ ...config, regions });
  };

  return (
    <>
      <FieldCheckbox
        mb={2}
        size="small"
        label={label}
        checked={config.enabled}
        onChange={toggle}
      />
      {config.enabled && (
        <Box ml={4}>
          <Box mb={3}>
            <RegionMultiSelector
              regionGroups={awsRegionGroups}
              selectedRegions={config.regions}
              onChange={handleRegionsChange}
              disabled={false}
              required={!allowWildcardRegions}
              rule={requiredRegions(allowWildcardRegions)}
              allowAllRegions={allowWildcardRegions}
            />
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
