/*
Copyright 2020 Gravitational, Inc.

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
import { Flex } from 'design';
import ConnectorListItem from './ConnectorListItem';

export default function ConnectorList({ items, onEdit, onDelete, ...styles }) {
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
    );
  });

  const { flex } = styles;
  return (
    <Flex flexWrap="wrap" alignItems="center" flex={flex}>
      {$items}
    </Flex>
  );
}
