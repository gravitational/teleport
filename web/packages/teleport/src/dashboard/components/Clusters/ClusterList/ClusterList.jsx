/*
Copyright 2019-2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { sortBy } from 'lodash';
import isMatch from 'design/utils/match';
import CardEmpty from 'teleport/components/CardEmpty';

import TableView from './TableView';

export default function ClustersList(props) {
  const { clusters, filter = '', pageSizeTable = 500 } = props;
  const filtered = sortAndFilter(clusters, filter);

  if (filtered.length === 0 && !!filter) {
    return (
      <CardEmpty mt="10" title={`No Results Found for "${filter}"`}></CardEmpty>
    );
  }

  return <TableView clusters={filtered} pageSize={pageSizeTable} />;
}

function sortAndFilter(clusters, searchValue) {
  const filtered = clusters.filter(obj =>
    isMatch(obj, searchValue, {
      searchableProps: [
        'clusterId',
        'version',
        'status',
        'connectedText',
        'labels',
      ],
      cb: searchAndFilterCb,
    })
  );

  // sort by date before grouping
  return sortBy(filtered, ['clusterId']);
}

function searchAndFilterCb(targetValue, searchValue, propName) {
  if (propName === 'labels') {
    return targetValue.some(item => {
      const { name, value } = item;
      return (
        name.toLocaleUpperCase().indexOf(searchValue) !== -1 ||
        value.toLocaleUpperCase().indexOf(searchValue) !== -1
      );
    });
  }
}
