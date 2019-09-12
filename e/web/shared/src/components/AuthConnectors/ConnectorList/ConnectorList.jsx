import React from 'react';
import { Flex  } from 'design';
import ConnectorListItem from './ConnectorListItem';

export default function ConnectorList({items, onEdit, onDelete, ...styles}){
  items = items || [];
  const $items = items.map(item => {
    const { id, name, kind } = item;
    return (
      <ConnectorListItem
        mb={4}
        mr={5}
        key={id}
        id={id}
        onEdit={onEdit}
        onDelete={onDelete}
        name={name}
        kind={kind}
      />
    )
  });

  const { flex } = styles;
  return (
    <Flex flexWrap="wrap" alignItems="center" flex={flex}>
      {$items}
    </Flex>
  )
}


