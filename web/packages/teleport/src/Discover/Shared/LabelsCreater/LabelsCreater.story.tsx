/**
 * Copyright 2022 Gravitational, Inc.
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

import { LabelsCreater, DiscoverLabel } from './LabelsCreater';

export default {
  title: 'Teleport/Discover/Shared/LabelsCreator',
};

export const OptionalLabels = () => {
  const [labels, setLabels] = useState([]);

  return (
    <Validation>
      <LabelsCreater
        labels={labels}
        setLabels={setLabels}
        isLabelOptional={true}
      />
    </Validation>
  );
};

export const RequiredLabels = () => {
  const [labels, setLabels] = useState([
    { name: 'env', value: 'prod' },
    { name: 'os', value: 'windows' },
  ]);

  return (
    <Validation>
      <LabelsCreater labels={labels} setLabels={setLabels} />
    </Validation>
  );
};

export const FixedLabels = () => {
  const [labels, setLabels] = useState<DiscoverLabel[]>([
    { name: 'env', value: 'prod', isFixed: true },
    { name: 'fruit', value: 'banana', isFixed: true },
    { name: 'os', value: 'windows' },
  ]);

  return (
    <Validation>
      <LabelsCreater labels={labels} setLabels={setLabels} />
    </Validation>
  );
};

export const DisabledAddButton = () => {
  const [labels, setLabels] = useState([{ name: '*', value: '*' }]);

  return (
    <Validation>
      <LabelsCreater labels={labels} setLabels={setLabels} disableBtns={true} />
    </Validation>
  );
};
