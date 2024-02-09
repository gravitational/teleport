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

import styled, { keyframes } from 'styled-components';
import { Button } from 'design';
import { Wand } from 'design/Icon';

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
    >
      <Wand
        css={`
          animation: ${rainbowColor} 3s linear infinite;
        `}
        size="medium"
      />
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

const rainbowColor = keyframes`
  100%,0%{
    color: rgb(255,0,0);
  }
  8%{
    color: rgb(255,127,0);
  }
  16%{
    color: rgb(255,255,0);
  }
  25%{
    color: rgb(127,255,0);
  }
  33%{
    color: rgb(0,255,0);
  }
  41%{
    color: rgb(0,255,127);
  }
  50%{
    color: rgb(0,255,255);
  }
  58%{
    color: rgb(0,127,255);
  }
  66%{
    color: rgb(0,0,255);
  }
  75%{
    color: rgb(127,0,255);
  }
  83%{
    color: rgb(255,0,255);
  }
  91%{
    color: rgb(255,0,127);
  }
`;
