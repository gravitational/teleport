/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

export const Mark = styled.mark`
  padding: 2px 5px;
  border-radius: 6px;
  font-family: ${p => p.theme.fonts.mono};
  font-size: ${p => p.theme.fontSizes[1]}px;
  background-color: ${p => p.theme.colors.interactive.tonal.neutral[2]};
  color: inherit;
`;

/**
 * Returns a MarkInverse that inverts the colors from its parent Mark.
 * For example, if current theme is dark theme, parent Mark would use
 * light colors, but MarkInverse will use dark colors.
 *
 * Intended for use in tooltips since tooltips uses inverse background
 * color of the current theme.
 */
export const MarkInverse = styled(Mark)`
  background-color: ${p => p.theme.colors.tooltip.inverseBackground};
  color: ${p => p.theme.colors.text.main};
`;
