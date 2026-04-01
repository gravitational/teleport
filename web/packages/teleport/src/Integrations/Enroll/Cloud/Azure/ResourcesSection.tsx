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

import { useMemo } from 'react';

import { Box, Flex, Text } from 'design';
import { FieldCheckbox } from 'shared/components/FieldCheckbox';
import { FieldMultiInput } from 'shared/components/FieldMultiInput/FieldMultiInput';
import { type Option } from 'shared/components/Select';
import {
  arrayOf,
  requiredAzureSubscriptionId,
  requiredAzureTagName,
  requiredField,
} from 'shared/components/Validation/rules';

import {
  LabelsInput,
  type Label,
  type LabelsRule,
} from 'teleport/components/LabelsInput/LabelsInput';
import { AzureRegion } from 'teleport/services/integrations';

import { CircleNumber } from '../Shared/common';
import { RegionSelect } from '../Shared/RegionSelect';
import { azureRegionOptions } from './regions';
import { AzureTag, VmConfig } from './types';

const subscriptionRule = (values: string[]) => () => {
  if (values.length === 0) {
    return {
      valid: false,
      results: [
        { valid: false, message: 'At least one subscription is required' },
      ],
    };
  }
  return arrayOf(requiredAzureSubscriptionId)(values)();
};

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
        config={vmConfig}
        onUpdate={patch => onVmChange({ ...vmConfig, ...patch })}
      />
    </>
  );
}

function AzureService({
  label,
  config,
  onUpdate,
}: {
  label: string;
  config: VmConfig;
  onUpdate: (patch: Partial<VmConfig>) => void;
}) {
  const isWildcardRegion = config.regions.some(r => r === '*');

  const selectedOptions = useMemo<Option<AzureRegion>[]>(() => {
    if (isWildcardRegion) return [];
    return azureRegionOptions.filter(opt => config.regions.includes(opt.value));
  }, [config.regions, isWildcardRegion]);

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
          <Box mb={3} maxWidth={432}>
            <FieldMultiInput
              label="Match subscriptions"
              required
              tooltipContent="Match by Azure subscription IDs to limit discovery to specific subscriptions. At least one subscription ID is required."
              value={config.subscriptions}
              placeholder="11111111-2222-3333-4444-555555555555"
              onChange={subscriptions => onUpdate({ subscriptions })}
              rule={subscriptionRule}
            />
          </Box>
          <Box mb={3} maxWidth={432}>
            <FieldMultiInput
              label="Match resource groups"
              tooltipContent="Match resources by resource group name to limit discovery to specific Azure resource groups. Leave empty to discover all resource groups."
              value={config.resourceGroups}
              placeholder="my-resource-group"
              onChange={resourceGroups => onUpdate({ resourceGroups })}
            />
          </Box>
          <Box width={400} mb={3}>
            <RegionSelect
              isMulti={true}
              label="Match regions"
              toolTipContent="Match resources in the specified regions to limit discovery to specific regions. Leave empty to discover all regions."
              options={azureRegionOptions}
              value={selectedOptions}
              placeholder={<Text color="text.main">All regions</Text>}
              onChange={options =>
                onUpdate({
                  regions:
                    options.length === 0
                      ? ['*']
                      : options.map((opt: Option<AzureRegion>) => opt.value),
                })
              }
              isDisabled={false}
            />
          </Box>
          <Box mb={2} width={433}>
            <LabelsInput
              legend="Match tags"
              adjective="tag"
              tooltipContent="Match resources by tag to limit discovery to specific tags."
              labels={config.tags as Label[]}
              labelKey={{ fieldName: 'Name', placeholder: 'Environment' }}
              labelVal={{ fieldName: 'Value', placeholder: 'production' }}
              setLabels={(tags: Label[]) =>
                onUpdate({ tags: tags as AzureTag[] })
              }
              rule={azureTagRule}
            />
          </Box>
        </Box>
      )}
    </>
  );
}
