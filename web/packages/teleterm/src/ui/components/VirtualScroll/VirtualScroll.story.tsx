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

import React from 'react';
import styled from 'styled-components';

import { Text } from 'design';

import { unique } from 'teleterm/ui/utils/uid';

import { VirtualScroll } from './VirtualScroll';

const items: SimpleItem[] = new Array(10000)
  .fill(null)
  .map((_, index) => ({ index }));

export default {
  title: 'Teleterm/components/VirtualScroll',
};

interface SimpleItem {
  index: number;
}

export function SimpleList() {
  return (
    <>
      <h2>Rendering {items.length} items</h2>
      <Container>
        <VirtualScroll<SimpleItem>
          keyProp="index"
          rowHeight={24}
          items={items}
          Node={({ item }) => <Text>This is item with {item.index} index</Text>}
        />
      </Container>
    </>
  );
}

interface TreeItem {
  children?: TreeItem[];
  id: string;
}

export function TreeList() {
  const depth = 6;
  const childrenPerLevel = 6;

  function createTree(currentLevel = 0): TreeItem {
    const item = {
      children: [],
      id: unique(),
    };

    if (currentLevel === depth) {
      return item;
    }

    for (let i = 0; i < childrenPerLevel; i++) {
      item.children.push(createTree(currentLevel + 1));
    }

    return item;
  }

  return (
    <>
      <h2>Rendering up to {Math.pow(depth, childrenPerLevel)} items</h2>
      <Container>
        <VirtualScroll<TreeItem>
          keyProp="id"
          childrenProp="children"
          rowHeight={24}
          items={[createTree()]}
          Node={({ item, isLeaf, onToggle, isExpanded, depth }) => {
            const marginLeft = depth * 15 + 'px';
            return (
              <Text>
                <ClickableItem
                  onClick={onToggle}
                  ml={marginLeft}
                  hidden={isLeaf}
                >
                  {isExpanded ? '-' : '+'}{' '}
                </ClickableItem>
                <span>This is item with {item.id} id</span>
              </Text>
            );
          }}
        />
      </Container>
    </>
  );
}

const ClickableItem = styled.span<{ ml: string }>`
  visibility: ${props => (props.hidden ? 'hidden' : 'visible')};
  margin-left: ${props => props.ml};
  font-weight: bold;
  cursor: pointer;
  display: inline-block;
  width: 10px;
  margin-right: 5px;

  &:hover {
    transform: scale(1.5);
  }
`;

const Container = styled.div`
  height: 500px;
`;
