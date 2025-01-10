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

import { forwardRef } from 'react';
import styled from 'styled-components';

import { Text } from 'design';
import { ChevronDown, ChevronUp } from 'design/Icon';

import { useKeyboardShortcutFormatters } from 'teleterm/ui/services/keyboardShortcuts';

interface ClusterSelectorProps {
  clusterName?: string;
  isOpened: boolean;

  onClick(): void;
}

export const ClusterSelector = forwardRef<
  HTMLButtonElement,
  ClusterSelectorProps
>((props, ref) => {
  const { getLabelWithAccelerator } = useKeyboardShortcutFormatters();
  const SortIcon = props.isOpened ? ChevronUp : ChevronDown;
  const text = props.clusterName || 'Select Cluster';

  return (
    <Container
      ref={ref}
      onClick={props.onClick}
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
      <SortIcon size="small" ml={3} />
    </Container>
  );
});

const Container = styled.button<{ isClusterSelected?: boolean }>`
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
