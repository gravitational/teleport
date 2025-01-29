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

import { color, space } from 'design/system';

function Link({ ...props }) {
  return <StyledButtonLink {...props} />;
}

Link.displayName = 'Link';

const StyledButtonLink = styled.a.attrs({
  rel: 'noreferrer',
})`
  color: ${({ theme }) => theme.colors.buttons.link.default};
  background: none;
  text-decoration: underline;
  text-transform: none;

  ${space}
  ${color}
`;

export default Link;
