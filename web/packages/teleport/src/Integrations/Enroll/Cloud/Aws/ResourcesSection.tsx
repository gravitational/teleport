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
import styled from 'styled-components';

import { Box, ButtonText, Flex, Text, Toggle } from 'design';
import * as Icons from 'design/Icon';
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

import { CircleNumber } from '../Shared';
import { RegionSelect } from '../Shared/RegionSelect';
import { awsRegionOptionGroups, awsRegionOptions } from './regions';
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
  (allowAll: boolean) => (options: readonly Option<AwsRegion>[]) => () => {
    if (allowAll || (options && options.length > 0)) {
      return { valid: true };
    }
    return {
      valid: false,
      message: 'At least one region must be selected',
    };
  };

const fieldWidth = 400;
// tags needs additional room for the close X on the right.
const tagsWidth = fieldWidth + 32;

type ResourcesSectionProps = {
  configs: ServiceConfigs;
  onConfigChange: (type: ServiceType, patch: Partial<ServiceConfig>) => void;
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

      <AwsService
        label="EC2 Instances"
        config={configs.ec2}
        onUpdate={patch => onConfigChange('ec2', patch)}
        allowAllRegions={true}
        tagTooltip="Match EC2 instances by their tags. If no tags are added, Teleport will match and enroll all EC2 instances."
      />

      <Box mt={3}>
        <AwsService
          label="EKS Clusters"
          config={configs.eks}
          onUpdate={patch => onConfigChange('eks', patch)}
          allowAllRegions={false}
          tagTooltip="Match EKS clusters by their tags. If no tags are added, Teleport will match and enroll all EKS clusters."
        >
          <Box mt={2}>
            <Toggle
              isToggled={configs.eks.kubeAppDiscovery}
              onToggle={() =>
                onConfigChange('eks', {
                  kubeAppDiscovery: !configs.eks.kubeAppDiscovery,
                })
              }
            >
              <Box ml={2} mr={1}>
                Enable Kubernetes App Discovery
              </Box>
              <IconTooltip kind="info">
                <Text>
                  Teleport's Kubernetes App Discovery will automatically
                  identify and enroll HTTP applications running inside
                  discovered Kubernetes clusters.
                </Text>
              </IconTooltip>
            </Toggle>
          </Box>
        </AwsService>
      </Box>
    </>
  );
}

function AwsService({
  label,
  config,
  onUpdate,
  allowAllRegions,
  tagTooltip,
  children,
}: {
  label: string;
  config: ServiceConfig;
  onUpdate: (update: Partial<ServiceConfig>) => void;
  allowAllRegions: boolean;
  tagTooltip: string;
  children?: React.ReactNode;
}) {
  const [tagsExpanded, setTagsExpanded] = useState(config.tags.length > 0);

  const selectedOptions: Option<AwsRegion>[] = useMemo(
    () => awsRegionOptions.filter(opt => config.regions.includes(opt.value)),
    [config.regions]
  );

  return (
    <>
      <FieldCheckbox
        mb={2}
        size="small"
        label={label}
        checked={config.enabled}
        onChange={() => onUpdate({ enabled: !config.enabled })}
      />
      {config.enabled && (
        <Box ml={4}>
          <Box mb={3} width={fieldWidth}>
            <RegionSelect
              isMulti={true}
              options={awsRegionOptionGroups}
              value={selectedOptions}
              onChange={options =>
                onUpdate({
                  regions: options.map(opt => opt.value),
                })
              }
              isDisabled={false}
              required={!allowAllRegions}
              rule={requiredRegions(allowAllRegions)}
              placeholder={
                allowAllRegions ? 'All regions' : 'Select regions...'
              }
            />
          </Box>
          <TagsButton onClick={() => setTagsExpanded(prev => !prev)}>
            <Flex alignItems="center" gap={1} mb={2}>
              <TagsChevron size="small" expanded={tagsExpanded} />
              Match by tag
              <IconTooltip kind="info">
                <Text>{tagTooltip}</Text>
              </IconTooltip>
            </Flex>
          </TagsButton>
          {tagsExpanded && (
            <Box mb={2}>
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
      )}
    </>
  );
}

const TagsButton = styled(ButtonText)`
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

const TagsChevron = styled(Icons.ChevronRight)<{
  expanded: boolean;
}>`
  transition: transform 0.2s ease-in-out;
  transform: ${props => (props.expanded ? 'rotate(90deg)' : 'none')};
`;
