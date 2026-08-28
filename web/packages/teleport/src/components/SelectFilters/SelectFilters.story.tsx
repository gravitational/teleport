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
