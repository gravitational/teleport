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
  min-width: 0;
  width: 100%;
  height: 100%;
  border: 0.5px ${props => props.theme.colors.action.disabledBackground} solid;
  border-radius: 4px;
  display: flex;
  flex-grow: 1;
  justify-content: space-between;
  align-items: center;
  padding: 0 12px;
  opacity: ${props => (props.isClusterSelected ? 1 : 0.6)};
  cursor: pointer;

  &:hover,
  &:focus {
    opacity: 1;
    border-color: ${props => props.theme.colors.light};
  }

  ${props => {
    if (props.isOpened) {
      return {
        borderColor: props.theme.colors.brand,
        opacity: 1,
      };
    }
  }}
`;
