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
import { Flex, Label, Text } from 'design';

import styled from 'styled-components';

import { ListItem } from 'teleterm/ui/components/ListItem';
import { useKeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation';
import { Cluster } from 'teleterm/services/tshd/types';

interface ClusterItemProps {
  index: number;
  item: Cluster;
  isSelected: boolean;
  onSelect(): void;
}

export function ClusterItem(props: ClusterItemProps) {
  const { isActive } = useKeyboardArrowsNavigation({
    index: props.index,
    onRun: props.onSelect,
  });

  const clusterName = props.item.name;

  return (
    <StyledListItem
      onClick={props.onSelect}
      isActive={isActive}
      isSelected={props.isSelected}
      isLeaf={props.item.leaf}
    >
      <Flex
        alignItems="center"
        justifyContent="space-between"
        flex="1"
        width="100%"
        minWidth="0"
      >
        <Text typography="body1" title={clusterName}>
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

const StyledListItem = styled(ListItem)`
  padding-left: ${props => (props.isLeaf ? '32px' : null)};

  &:hover,
  &:focus {
    background: ${props => props.theme.colors.spotBackground[0]};
  }
`;
