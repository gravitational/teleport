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

import { Add } from 'design/Icon';

import styled from 'styled-components';

import { useKeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation';
import { ListItem } from 'teleterm/ui/components/ListItem';

interface AddNewClusterItemProps {
  index: number;

  onClick(): void;
}

export function AddNewClusterItem(props: AddNewClusterItemProps) {
  const { isActive } = useKeyboardArrowsNavigation({
    index: props.index,
    onRun: props.onClick,
  });

  return (
    <StyledListItem isActive={isActive} onClick={props.onClick}>
      <Add mr={1} color="inherit" />
      Add another cluster
    </StyledListItem>
  );
}

const StyledListItem = styled(ListItem)`
  border-radius: 0;
  height: 38px;
  justify-content: center;
  color: ${props => props.theme.colors.text.slightlyMuted};
`;
