/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { Flex, Label, Text } from 'design';
import { CircleCheck } from 'design/Icon';

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

  const LabelVersion = props.isSelected ? InvertedLabel : Label;
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
        {!props.item.leaf ? (
          <LabelVersion ml={1} kind="primary">
            root
          </LabelVersion>
        ) : null}
        {props.isSelected ? <CircleCheck fontSize={12} /> : null}
      </Flex>
    </StyledListItem>
  );
}

const StyledListItem = styled(ListItem)`
  padding-left: ${props => (props.isLeaf ? '32px' : null)};
  background: ${getBackgroundColor};

  &:hover,
  &:focus {
    background: ${getHoverBackgroundColor};
  }
`;

const InvertedLabel = styled(Label)`
  color: ${props => props.theme.colors.brand};
  background-color: ${props => props.theme.colors.text.contrast};
`;

function getBackgroundColor(props) {
  if (props.isSelected) {
    if (props.isActive) {
      return props.theme.colors.brandAccent;
    }
    return props.theme.colors.brand;
  }
}

function getHoverBackgroundColor(props) {
  if (props.isSelected) {
    return props.theme.colors.brandAccent;
  }
}
