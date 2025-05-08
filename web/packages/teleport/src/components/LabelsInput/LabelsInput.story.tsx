/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { Meta } from '@storybook/react';
import { useState } from 'react';

import Box from 'design/Box';
import { ButtonSecondary } from 'design/Button';
import Validation from 'shared/components/Validation';

import { Label, LabelsInput } from './LabelsInput';

const meta: Meta = {
  title: 'Teleport/LabelsInput',
  decorators: Story => (
    <Box width="456px">
      <Story />
    </Box>
  ),
};
export default meta;

export const Default = () => {
  const [labels, setLabels] = useState<Label[]>([]);

  return (
    <Validation>
      {({ validator }) => (
        <div>
          <div>
            <LabelsInput labels={labels} setLabels={setLabels} />
          </div>
          <ButtonSecondary
            mt={6}
            onClick={() => {
              if (!validator.validate()) {
                return;
              }
            }}
          >
            Test Validation For Empty Inputs
          </ButtonSecondary>
        </div>
      )}
    </Validation>
  );
};

export const Custom = () => {
  const [labels, setLabels] = useState<Label[]>([]);
  return (
    <Validation>
      <LabelsInput
        labels={labels}
        setLabels={setLabels}
        legend="List of Labels"
        tooltipContent="List of labels, 'nuff said"
        labelKey={{
          fieldName: 'Custom Key Name',
          placeholder: 'custom key placeholder',
        }}
        labelVal={{
          fieldName: 'Custom Value',
          placeholder: 'custom value placeholder',
        }}
        adjective="Custom Adjective"
      />
    </Validation>
  );
};

export const Disabled = () => {
  const [labels, setLabels] = useState<Label[]>([
    { name: 'some-name', value: 'some-value' },
  ]);
  return (
    <Validation>
      <LabelsInput labels={labels} setLabels={setLabels} disableBtns={true} />
    </Validation>
  );
};

export const AutoFocus = () => {
  const [labels, setLabels] = useState<Label[]>([{ name: '', value: '' }]);
  return (
    <Validation>
      <LabelsInput labels={labels} setLabels={setLabels} autoFocus={true} />
    </Validation>
  );
};

export const AtLeastOneRequired = () => {
  const [labels, setLabels] = useState<Label[]>([{ name: '', value: '' }]);
  return (
    <Validation>
      <LabelsInput
        legend="Labels"
        labels={labels}
        setLabels={setLabels}
        required={true}
      />
    </Validation>
  );
};

export const AtLeastOneRowVisible = () => {
  const [labels, setLabels] = useState<Label[]>([]);
  return (
    <Validation>
      <LabelsInput
        legend="Labels"
        labels={labels}
        setLabels={setLabels}
        atLeastOneRow
      />
    </Validation>
  );
};
