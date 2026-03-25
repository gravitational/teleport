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

import styled from 'styled-components';

import { Box, ButtonText, Flex, Text } from 'design';
import * as Icons from 'design/Icon';
import { IconTooltip } from 'design/Tooltip';
import { FieldCheckbox } from 'shared/components/FieldCheckbox';
import { requiredField } from 'shared/components/Validation/rules';

import {
  LabelsInput,
  type Label,
  type LabelsRule,
} from 'teleport/components/LabelsInput/LabelsInput';

import { CircleNumber } from './EnrollAws';
import { AwsLabel, Ec2Config } from './types';

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

type ResourcesSectionProps = {
  ec2Config: Ec2Config;
  onEc2Change: (config: Ec2Config) => void;
};

export function ResourcesSection({
  ec2Config,
  onEc2Change,
}: ResourcesSectionProps) {
  return (
    <>
      <Flex alignItems="center" fontSize={4} fontWeight="medium" mb={1}>
        <CircleNumber>3</CircleNumber>
        Resource Types
      </Flex>
      <Text ml={4} mb={3}>
        Select which AWS resource types to automatically discover and enroll.
      </Text>
      <AwsService
        label="EC2 Instances"
        helperText={
          <Text>
            Discover EC2 instances and establish SSH access through the Teleport
            proxy.
            <br />
            Note: If no tags are specified, all EC2 instances in the selected
            regions will be enrolled.
          </Text>
        }
        tooltipText="Filter for EC2 instances by their tags. If no tags are added, Teleport will enroll all EC2 instances."
        config={ec2Config}
        onChange={onEc2Change}
      />
    </>
  );
}

type ServiceConfig = Ec2Config;

type AwsServiceProps = {
  label: string;
  helperText: React.ReactNode;
  tooltipText: string;
  config: ServiceConfig;
  onChange: (config: ServiceConfig) => void;
};

function AwsService({
  label,
  helperText,
  tooltipText,
  config,
  onChange,
}: AwsServiceProps) {
  const toggle = () => {
    onChange({
      ...config,
      enabled: !config.enabled,
    });
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
      <Text fontSize="small" ml={4}>
        Note
      </Text>
      <Box ml={4}>
        <FilterButton onClick={toggle}>
          <Flex alignItems="center" gap={1} mb={2}>
            <FilterChevron size="small" expanded={config.enabled} />
            Filter by tag
            <IconTooltip kind="info">
              <Text>{tooltipText}</Text>
            </IconTooltip>
          </Flex>
        </FilterButton>
        {config.enabled && (
          <Box mb={2}>
            <Box width={400}>
              <LabelsInput
                adjective="tag"
                labels={config.tags as Label[]}
                labelKey={{ fieldName: 'Key', placeholder: 'Environment' }}
                labelVal={{ fieldName: 'Value', placeholder: 'production' }}
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

const FilterChevron = styled(Icons.ChevronRight)<{ expanded: boolean }>`
  transition: transform 0.2s ease-in-out;
  transform: ${props => (props.expanded ? 'rotate(90deg)' : 'none')};
`;
