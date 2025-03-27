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

import { useState } from 'react';
import styled from 'styled-components';

import Box from 'design/Box';
import { ButtonSecondary } from 'design/Button';
import Validation from 'shared/components/Validation';

import { Label, LabelsInput } from './LabelsInput';

export default {
  title: 'Teleport/LabelsInput',
};

export const Default = () => {
  const [labels, setLables] = useState<Label[]>([]);

  return (
    <Wrapper>
      <Validation>
        {({ validator }) => (
          <div>
            <div>
              <LabelsInput labels={labels} setLabels={setLables} />
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
    </Wrapper>
  );
};

export const Custom = () => {
  const [labels, setLables] = useState<Label[]>([]);
  return (
    <Wrapper>
      <Validation>
        <LabelsInput
          labels={labels}
          setLabels={setLables}
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
    </Wrapper>
  );
};

export const Disabled = () => {
  const [labels, setLables] = useState<Label[]>([
    { name: 'some-name', value: 'some-value' },
  ]);
  return (
    <Wrapper>
      <Validation>
        <LabelsInput labels={labels} setLabels={setLables} disableBtns={true} />
      </Validation>
    </Wrapper>
  );
};

export const AutoFocus = () => {
  const [labels, setLables] = useState<Label[]>([{ name: '', value: '' }]);
  return (
    <Wrapper>
      <Validation>
        <LabelsInput labels={labels} setLabels={setLables} autoFocus={true} />
      </Validation>
    </Wrapper>
  );
};

export const AtLeastOneRequired = () => {
  const [labels, setLables] = useState<Label[]>([{ name: '', value: '' }]);
  return (
    <Wrapper>
      <Validation>
        <LabelsInput
          legend="Labels"
          labels={labels}
          setLabels={setLables}
          required={true}
        />
      </Validation>
    </Wrapper>
  );
};

const Wrapper = styled(Box)`
  width: 456px;
`;
