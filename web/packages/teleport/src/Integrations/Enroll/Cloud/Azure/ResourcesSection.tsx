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

import { Box, Flex, Text } from 'design';
import { IconTooltip } from 'design/Tooltip';
import { FieldCheckbox } from 'shared/components/FieldCheckbox';
import {
  requiredField,
  requiredAzureTagName,
} from 'shared/components/Validation/rules';

import {
  LabelsInput,
  type Label,
  type LabelsRule,
} from 'teleport/components/LabelsInput/LabelsInput';

import {
  CircleNumber,
  FilterButton,
  FilterChevron,
} from '../Shared/common';
import { AzureTag, VmConfig } from './types';

const azureTagRule: LabelsRule = (labels: Label[]) => () => {
  const results = labels.map(label => ({
    name: requiredAzureTagName(label.name)(),
    value: requiredField('Please enter a tag value')(label.value)(),
  }));
  return {
    valid: results.every(r => r.name.valid && r.value.valid),
    results: results,
  };
};

type ResourcesSectionProps = {
  vmConfig: VmConfig;
  onVmChange: (config: VmConfig) => void;
};

export function ResourcesSection({
  vmConfig,
  onVmChange,
}: ResourcesSectionProps) {
  return (
    <>
      <Flex alignItems="center" fontSize={4} fontWeight="medium" mb={1}>
        <CircleNumber>3</CircleNumber>
        Resource Types
      </Flex>
      <Text ml={4} mb={3}>
        Select which Azure resource types to automatically discover and enroll.
      </Text>
      <AzureService
        label="Virtual Machines"
        helperText={
          <Text>
            Discover Azure VM instances and establish SSH access through the
            Teleport proxy.
            <br />
            Note: If no tags are specified, all Azure VMs in the selected
            regions will be enrolled.
          </Text>
        }
        tooltipText="Filter for Azure VMs by their tags. If no tags are added, Teleport will enroll all instances."
        config={vmConfig}
        onChange={onVmChange}
      />
    </>
  );
}

type ServiceConfig = VmConfig;

type AzureServiceProps = {
  label: string;
  helperText: React.ReactNode;
  tooltipText: string;
  config: ServiceConfig;
  onChange: (config: ServiceConfig) => void;
};

function AzureService({
  label,
  helperText,
  tooltipText,
  config,
  onChange,
}: AzureServiceProps) {
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
            <Box width={600}>
              <LabelsInput
                adjective="tag"
                labels={config.tags as Label[]}
                labelKey={{ fieldName: 'Name', placeholder: 'Environment' }}
                labelVal={{ fieldName: 'Value', placeholder: 'production' }}
                setLabels={(tags: Label[]) =>
                  onChange({
                    ...config,
                    tags: tags as AzureTag[],
                  })
                }
                rule={azureTagRule}
              />
            </Box>
          </Box>
        )}
      </Box>
    </>
  );
}
