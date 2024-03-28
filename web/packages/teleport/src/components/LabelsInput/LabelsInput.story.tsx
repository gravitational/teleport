/**
 * Copyright 2024 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useState } from 'react';
import Validation from 'shared/components/Validation';
import { ButtonSecondary } from 'design/Button';

import { LabelsInput, Label } from './LabelsInput';

export default {
  title: 'Teleport/LabelsInput',
};

export const Default = () => {
  const [labels, setLables] = useState<Label[]>([]);

  return (
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
  );
};

export const Custom = () => {
  const [labels, setLables] = useState<Label[]>([]);
  return (
    <Validation>
      <LabelsInput
        labels={labels}
        setLabels={setLables}
        labelKey={{
          fieldName: 'Custom Key Name',
          placeholder: 'custom key placeholder',
        }}
        labelVal={{
          fieldName: 'Custom Value',
          placeholder: 'custom value placeholder',
        }}
        adjective="Custom Adjective"
        inputWidth={350}
      />
    </Validation>
  );
};

export const Disabled = () => {
  const [labels, setLables] = useState<Label[]>([
    { name: 'some-name', value: 'some-value' },
  ]);
  return (
    <Validation>
      <LabelsInput labels={labels} setLabels={setLables} disableBtns={true} />
    </Validation>
  );
};

export const AutoFocus = () => {
  const [labels, setLables] = useState<Label[]>([{ name: '', value: '' }]);
  return (
    <Validation>
      <LabelsInput labels={labels} setLabels={setLables} autoFocus={true} />
    </Validation>
  );
};

export const AtLeastOneRequired = () => {
  const [labels, setLables] = useState<Label[]>([{ name: '', value: '' }]);
  return (
    <Validation>
      <LabelsInput
        labels={labels}
        setLabels={setLables}
        areLabelsRequired={true}
      />
    </Validation>
  );
};
