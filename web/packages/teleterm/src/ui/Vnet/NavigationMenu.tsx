/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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
import { Button } from 'design';

import { useWorkspaceContext } from 'teleterm/ui/Documents';

export function NavigationMenu() {
  const { documentsService, rootClusterUri } = useWorkspaceContext();

  function openDocument(): void {
    documentsService.openVnetDocument({ rootClusterUri });
  }

  return (
    <StyledButton
      onClick={openDocument}
      kind="secondary"
      size="small"
      title="Open VNet"
      textTransform="none"
    >
      {/* TODO(ravicious): Replace it with an icon. */}
      VNet
    </StyledButton>
  );
}

const StyledButton = styled(Button)`
  position: relative;
  background: ${props => props.theme.colors.spotBackground[0]};
  padding: 0;
  width: ${props => props.theme.space[5]}px;
  height: ${props => props.theme.space[5]}px;
`;
