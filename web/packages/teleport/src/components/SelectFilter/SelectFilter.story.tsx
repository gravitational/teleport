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
import SelectFilter, { Props } from './SelectFilter';

export default {
  title: 'Teleport/SelectFilter',
};

export const NoAppliedFilters = () => <SelectFilter {...props} />;
export const WithAppliedFilters = () => (
  <SelectFilter {...props} appliedFilters={props.filters} />
);

const props: Props = {
  applyFilters: () => null,
  appliedFilters: [],
  filters: [
    {
      value:
        'value_autoscaling-group: teleport-cloud-prod-us-west-2-worker-20201211102915999300000000002',
      label:
        'label_autoscaling-group: teleport-cloud-prod-us-west-2-worker-20201211102915999300000000002',
    },
    {
      value: 'value_country: United States of America',
      label: 'label_country: United States of America',
    },
    {
      value: 'value_country: France',
      label: 'label_country: France',
    },
    {
      value: 'value_kernel: 4.15.0-51-generic',
      label: 'label_kernel: 4.15.0-51-generic',
    },
    { value: 'value_os:  ubuntu', label: 'label_os:  ubuntu' },
    { value: 'value_os:  macOS', label: 'label_os:  macOS' },
    { value: 'value_owner:  username02', label: 'label_owner:  username02' },
    {
      value: 'value_tag-Name: teleport-cloud-dev-ap-northeast-1-controller',
      label: 'label_tag-Name: teleport-cloud-dev-ap-northeast-1-controller',
    },
    {
      value: 'value_zone: ap-northeast-1',
      label: 'label_zone: ap-northeast-1',
    },
    {
      value:
        'value_autoscaling-group: teleport-cloud-prod-us-west-2-master-20201211102915999300000000004',
      label:
        'label_autoscaling-group: teleport-cloud-prod-us-west-2-master-20201211102915999300000000004',
    },
  ],
};
