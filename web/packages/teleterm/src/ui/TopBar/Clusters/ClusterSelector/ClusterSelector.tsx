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

import React, { forwardRef } from 'react';
import { SortAsc, SortDesc } from 'design/Icon';
import styled from 'styled-components';
import { Text } from 'design';

import { useKeyboardShortcutFormatters } from 'teleterm/ui/services/keyboardShortcuts';

interface ClusterSelectorProps {
  clusterName?: string;
  isOpened: boolean;

  onClick(): void;
}

export const ClusterSelector = forwardRef<HTMLDivElement, ClusterSelectorProps>(
  (props, ref) => {
    const { getLabelWithAccelerator } = useKeyboardShortcutFormatters();
    const SortIcon = props.isOpened ? SortAsc : SortDesc;
    const text = props.clusterName || 'Select Cluster';

    return (
      <Container
        ref={ref}
        onClick={props.onClick}
        isOpened={props.isOpened}
        isClusterSelected={!!props.clusterName}
        title={getLabelWithAccelerator(
          [props.clusterName, 'Open Clusters'].filter(Boolean).join('\n'),
          'openClusters'
        )}
      >
        <Text
          css={`
            white-space: nowrap;
          `}
        >
          {text}
        </Text>
        <SortIcon fontSize={12} ml={3} />
      </Container>
    );
  }
);

const Container = styled.button`
  background: inherit;
  color: inherit;
  font-family: inherit;
  flex: 1;
  flex-shrink: 2;
  min-width: calc(${props => props.theme.space[7]}px * 2);
  height: 100%;
  border: 1px ${props => props.theme.colors.buttons.border.border} solid;
  border-radius: 4px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 ${props => props.theme.space[2]}px;
  opacity: ${props => (props.isClusterSelected ? 1 : 0.6)};
  cursor: pointer;

  &:hover,
  &:focus {
    opacity: 1;
    background: ${props => props.theme.colors.spotBackground[0]};
  }
`;
