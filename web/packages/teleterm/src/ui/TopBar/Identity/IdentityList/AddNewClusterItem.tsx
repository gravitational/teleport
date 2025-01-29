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

import styled from 'styled-components';

import { Add } from 'design/Icon';

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
      <Add mr={1} size="small" />
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
