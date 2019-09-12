import React from 'react';
import { sortBy } from 'lodash';
import isMatch from 'design/utils/match';
import CardEmpty from 'teleport/components/CardEmpty';
import GridView from './GridView';
import TableView from './TableView';
import { ModeEnum } from './../SwitchMode';

export default function ClustersList(props) {
  const {
    clusters,
    filter = '',
    mode,
    pageSizeGrid = 20,
    pageSizeTable = 500,
  } = props;
  const filtered = sortAndFilter(clusters, filter);

  if (filtered.length === 0 && !!filter) {
    return (
      <CardEmpty mt="10" title={`No Results Found for "${filter}"`}></CardEmpty>
    );
  }

  return (
    <>
      {mode === ModeEnum.GRID && (
        <GridView clusters={filtered} pageSize={pageSizeGrid} />
      )}
      {mode === ModeEnum.TABLE && (
        <TableView clusters={filtered} pageSize={pageSizeTable} />
      )}
    </>
  );
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
