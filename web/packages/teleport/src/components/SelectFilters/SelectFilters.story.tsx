/**
 * Copyright 2021 Gravitational, Inc.
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

import React from 'react';

import SelectFilter, { Props } from './SelectFilters';

export default {
  title: 'Teleport/SelectFilters',
};

export const Filters = () => <SelectFilter {...props} />;
export const FiltersWithPages = () => <SelectFilter {...props} pageSize={3} />;
export const WithAppliedFilters = () => (
  <SelectFilter {...props} appliedFilters={props.filters} />
);

const props: Props = {
  applyFilters: () => null,
  appliedFilters: [],
  filters: [
    {
      name: 'autoscaling-group',
      value:
        'teleport-cloud-prod-us-west-2-worker-2020121110291599930000000000200000000000',
      kind: 'label',
    },
    {
      name: 'country',
      value: 'United States of America',
      kind: 'label',
    },
    {
      name: 'country',
      value: 'France',
      kind: 'label',
    },
    {
      name: 'kernel',
      value: '4.15.0-51-generic',
      kind: 'label',
    },
    { name: 'os', value: 'ubuntu', kind: 'label' },
    { name: 'o', value: 'macOS', kind: 'label' },
    {
      name: 'owner',
      value: 'username02',
      kind: 'label',
    },
    {
      name: 'name',
      value: 'teleport-cloud-dev-ap-northeast-1-controller',
      kind: 'label',
    },
    {
      name: 'zone',
      value: 'ap-northeast-1',
      kind: 'label',
    },
    {
      name: 'autoscaling-group',
      value:
        'teleport-cloud-prod-us-west-2-master-20201211102915999300000000004',
      kind: 'label',
    },
  ],
};
