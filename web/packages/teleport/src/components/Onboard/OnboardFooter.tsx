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

import { Flex, Link, Text } from 'design';

export const OnboardFooter = () => {
  return (
    <StyledFooter>
      <StyledContent>
        <Text>&copy; Gravitational, Inc. All Rights Reserved</Text>
        <StyledLink href={'https://goteleport.com/legal/tos/'} target="_blank">
          Terms of Service
        </StyledLink>
        <StyledLink
          href={'https://goteleport.com/legal/privacy/'}
          target="_blank"
        >
          Privacy Policy
        </StyledLink>
      </StyledContent>
    </StyledFooter>
  );
};

const StyledContent = styled(Flex)`
  justify-content: center;
  width: 100%;
  gap: 50px;

  @media screen and (max-width: 800px) {
    flex-direction: column-reverse;
    text-align: center;
    gap: 10px;
  }
`;

const StyledFooter = styled('footer')`
  padding-bottom: ${props => props.theme.space[4]}px;
  width: 100%;
  color: ${props => props.theme.colors.text.main};
`;

const StyledLink = styled(Link)`
  color: ${props => props.theme.colors.text.main};
  text-decoration: none;

  &:hover,
  &:active,
  &:focus {
    color: ${props => props.theme.colors.text.muted};
  }
`;
