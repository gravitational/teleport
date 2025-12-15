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

import { useEffect, useRef } from 'react';
import styled from 'styled-components';

import { Flex, Label, Text } from 'design';

import { Cluster } from 'teleterm/services/tshd/types';
import { useKeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation';
import { ListItem } from 'teleterm/ui/components/ListItem';

interface ClusterItemProps {
  index: number;
  item: Cluster;
  isSelected: boolean;
  onSelect(): void;
}

export function ClusterItem(props: ClusterItemProps) {
  const { isActive, scrollIntoViewIfActive } = useKeyboardArrowsNavigation({
    index: props.index,
    onRun: props.onSelect,
  });
  const ref = useRef<HTMLLIElement>(null);

  const clusterName = props.item.name;

  useEffect(() => {
    scrollIntoViewIfActive(ref.current);
  }, [scrollIntoViewIfActive]);

  return (
    <StyledListItem
      ref={ref}
      onClick={props.onSelect}
      isActive={isActive}
      isLeaf={props.item.leaf}
    >
      <Flex
        alignItems="center"
        justifyContent="space-between"
        flex="1"
        width="100%"
        minWidth="0"
      >
        <Text typography="body2" title={clusterName}>
          {clusterName}
        </Text>
        <Flex>
          {!props.item.leaf ? (
            <Label ml={1} kind="primary">
              root
            </Label>
          ) : null}
          {props.isSelected ? (
            <Label ml={1} kind="success">
              active
            </Label>
          ) : null}
        </Flex>
      </Flex>
    </StyledListItem>
  );
}

const StyledListItem = styled(ListItem)<{ isLeaf?: boolean }>`
  padding-left: ${props => (props.isLeaf ? '32px' : null)};

  &:hover,
  &:focus {
    background: ${props => props.theme.colors.spotBackground[0]};
  }
`;
