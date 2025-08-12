/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import styled, { useTheme } from 'styled-components';

import Box from 'design/Box';
import Image from 'design/Image';

import logoPoweredByDark from './logoPoweredByDark.svg';
import logoPoweredByLight from './logoPoweredByLight.svg';

export function PoweredByTeleportLogo() {
  const theme = useTheme();
  const src = theme.type === 'dark' ? logoPoweredByDark : logoPoweredByLight;
  return (
    <StyledBox
      py={3}
      px={4}
      pb={4}
      css={`
        border: none;
      `}
    >
      <Image src={src} maxWidth="100%" alt="powered by teleport" />
    </StyledBox>
  );
}

const StyledBox = styled(Box)`
  line-height: 20px;
  border-top: ${props => props.theme.borders[1]}
    ${props => props.theme.colors.spotBackground[0]};
`;
