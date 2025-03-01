/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import Box from 'design/Box';
import Flex from 'design/Flex';
import Text from 'design/Text';
import FieldInput from 'shared/components/FieldInput';
import { FieldSelect } from 'shared/components/FieldSelect';
import { precomputed } from 'shared/components/Validation/rules';

import { LabelsInput } from 'teleport/components/LabelsInput';

import { SectionBox, SectionProps } from './sections';
import { MetadataModel, roleVersionOptions } from './standardmodel';
import { MetadataValidationResult } from './validation';

export const MetadataSection = ({
  value,
  isProcessing,
  validation,
  onChange,
}: SectionProps<MetadataModel, MetadataValidationResult>) => (
  <Flex flexDirection="column" gap={3}>
    Basic information about this role
    <SectionBox
      titleSegments={['Role Information']}
      isProcessing={isProcessing}
      validation={validation}
    >
      <FieldInput
        label="Role Name"
        required
        placeholder="Enter Role Name"
        value={value.name}
        disabled={isProcessing}
        rule={precomputed(validation.fields.name)}
        onChange={e => onChange({ ...value, name: e.target.value })}
      />
      <FieldInput
        label="Description"
        placeholder="Enter Role Description"
        value={value.description || ''}
        disabled={isProcessing}
        onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
          onChange({ ...value, description: e.target.value })
        }
      />
      <Box mb={3}>
        <Text typography="body3" mb={1}>
          Labels
        </Text>
        <LabelsInput
          disableBtns={isProcessing}
          labels={value.labels}
          setLabels={labels => onChange({ ...value, labels })}
          rule={precomputed(validation.fields.labels)}
        />
      </Box>
      <FieldSelect
        label="Version"
        isDisabled={isProcessing}
        options={roleVersionOptions}
        value={value.version}
        onChange={version => onChange({ ...value, version })}
        mb={0}
        menuPosition="fixed"
      />
    </SectionBox>
  </Flex>
);
