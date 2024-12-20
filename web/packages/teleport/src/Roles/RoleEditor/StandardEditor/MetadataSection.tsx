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

import FieldInput from 'shared/components/FieldInput';

import { precomputed } from 'shared/components/Validation/rules';

import { LabelsInput } from 'teleport/components/LabelsInput';

import Text from 'design/Text';

import { SectionBox, SectionProps } from './sections';
import { MetadataModel } from './standardmodel';
import { MetadataValidationResult } from './validation';

export const MetadataSection = ({
  value,
  isProcessing,
  validation,
  onChange,
}: SectionProps<MetadataModel, MetadataValidationResult>) => (
  <SectionBox
    title="Role Metadata"
    tooltip="Basic information about the role resource"
    isProcessing={isProcessing}
    validation={validation}
  >
    <FieldInput
      label="Role Name"
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
    <Text typography="body3" mb={1}>
      Labels
    </Text>
    <LabelsInput
      disableBtns={isProcessing}
      labels={value.labels}
      setLabels={labels => onChange?.({ ...value, labels })}
      rule={precomputed(validation.fields.labels)}
    />
  </SectionBox>
);
