/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import Validation from 'shared/components/Validation';

import { DiscoverLabel, LabelsCreater } from './LabelsCreater';

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

export const NoDuplicates = () => {
  const [labels, setLabels] = useState([]);

  return (
    <Validation>
      <LabelsCreater
        labels={labels}
        setLabels={setLabels}
        isLabelOptional={true}
        noDuplicateKey
      />
    </Validation>
  );
};
