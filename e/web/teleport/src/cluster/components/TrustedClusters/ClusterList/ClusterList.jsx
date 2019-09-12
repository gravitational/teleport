import React from 'react';
import { Flex } from 'design';
import ClusterListItem from './ClusterListItem';

export default function ClusterList({ items, onEdit, onDelete, ...styles }) {
  items = items || [];
  const $items = items.map(item => {
    const { id, name, kind } = item;
    return (
      <ClusterListItem
        mb={4}
        mr={5}
        key={id}
        id={id}
        onEdit={onEdit}
        onDelete={onDelete}
        name={name}
        kind={kind}
      />
    );
  });

  const { flex } = styles;
  return (
    <Flex flexWrap="wrap" alignItems="center" flex={flex}>
      {$items}
    </Flex>
  );
}
