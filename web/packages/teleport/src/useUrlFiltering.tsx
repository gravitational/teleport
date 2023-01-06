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

import { useMemo } from 'react';

import { makeLabelTag } from 'teleport/components/formatters';
import { Label, Filter } from 'teleport/types';

import useUrlQueryParams from './useUrlQueryParams';

export default function useUrlFiltering<T extends Filterable>(data: T[]) {
  const params = useUrlQueryParams();
  const filtered = useMemo(
    () => filterData(data, params.filters),
    [data, params.filters]
  );

  const labels = useMemo(() => getLabelFilters(data), [data]);

  return {
    result: filtered,
    filters: labels,
    appliedFilters: params.filters,
    applyFilters: params.applyFilters,
    toggleFilter: params.toggleFilter,
  };
}

function getLabelFilters<T extends Filterable>(data: T[] = []): Filter[] {
  // Test a labels field exist.
  if (!data.length || !data[0].labels) {
    return [];
  }

  // Extract unique labels.
  const tagDict: { [tag: string]: Label } = {};
  data.forEach(({ labels }) => {
    labels.forEach(label => {
      const tag = makeLabelTag(label);
      if (!tagDict[tag]) {
        tagDict[tag] = label;
      }
    });
  });

  const collator = new Intl.Collator(undefined, { numeric: true });
  return Object.keys(tagDict)
    .sort(collator.compare)
    .map(tag => ({
      name: tagDict[tag].name,
      value: tagDict[tag].value,
      kind: 'label',
    }));
}

// filterData returns new list of data that contains the selected labels.
function filterData<T extends Filterable>(
  data: T[] = [],
  filters: Filter[] = []
): T[] {
  if (!filters.length) {
    return data;
  }

  return data.filter(obj =>
    filters.every(filter => {
      switch (filter.kind) {
        case 'label':
          return obj.labels
            .map(makeLabelTag)
            .toString()
            .includes(makeLabelTag({ name: filter.name, value: filter.value }));
      }
    })
  );
}

export type Filterable = {
  labels: Label[];
};

export type State = ReturnType<typeof useUrlFiltering>;
