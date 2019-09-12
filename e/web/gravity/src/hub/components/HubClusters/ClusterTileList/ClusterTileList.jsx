import React from 'react';
import { sortBy } from 'lodash';
import { Flex } from 'design';
import ClusterTile from './ClusterTile';

export default function ClusterTileList({ clusters }) {
  // sort by date before grouping
  const sorted = sortBy(clusters, 'created').reverse();
  const $clusters = sorted.map(item => (
      <ClusterTile mr="5" mb="5" key={item.id} cluster={item} />
    ));

  return (
    <Flex flexWrap="wrap">
      {$clusters}
    </Flex>
  )
}