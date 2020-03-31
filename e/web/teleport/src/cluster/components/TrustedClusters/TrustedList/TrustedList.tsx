import React from 'react';
import { Flex } from 'design';
import TrustedListItem from './TrustedListItem';

export default function TrustedList({
  items,
  onEdit,
  onDelete,
  ...styles
}: Props) {
  items = items || [];
  const $items = items.map(item => {
    const { id, name, kind } = item;
    return (
      <TrustedListItem
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

  return (
    <Flex flexWrap="wrap" alignItems="center" {...styles}>
      {$items}
    </Flex>
  );
}

type Props = {
  items: any[];
  onEdit: (id: string) => void;
  onDelete: (id: string) => void;
  [index: string]: any;
};
