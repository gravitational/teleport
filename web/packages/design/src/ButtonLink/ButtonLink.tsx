/*
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

import { Button, ButtonProps } from 'design/Button';

function ButtonLink({ ...props }: ButtonProps<'a'>) {
  return <Button as={StyledButtonLink} {...props} />;
}

const StyledButtonLink = styled.a`
  color: ${({ theme }) => theme.colors.buttons.link.default};
  font-weight: normal;
  background: none;
  text-decoration: underline;
  text-transform: none;
  padding: 0 8px;

  &:hover,
  &:focus {
    color: ${({ theme }) => theme.colors.buttons.link.hover};
    box-shadow: none;
  }

  &:active {
    color: ${({ theme }) => theme.colors.buttons.link.active};
  }

  &:hover,
  &:focus,
  &:active {
    background: ${({ theme }) => theme.colors.levels.surface};
  }
`;

export default ButtonLink;
