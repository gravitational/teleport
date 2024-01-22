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

import {
  overflow,
  borders,
  borderRadius,
  borderColor,
  flex,
  height,
  lineHeight,
  maxWidth,
  minHeight,
  maxHeight,
  minWidth,
  alignSelf,
  justifySelf,
  space,
  width,
  color,
  textAlign,
} from '../system';

const Box = styled.div`
  box-sizing: border-box;
  ${maxWidth}
  ${minWidth}
  ${space}
  ${height}
  ${lineHeight}
  ${minHeight}
  ${maxHeight}
  ${width}
  ${color}
  ${textAlign}
  ${flex}
  ${alignSelf}
  ${justifySelf}
  ${borders}
  ${borderRadius}
  ${overflow}
  ${borderColor}
`;

Box.displayName = 'Box';

Box.propTypes = {
  ...space.propTypes,
  ...height.propTypes,
  ...width.propTypes,
  ...color.propTypes,
  ...textAlign.propTypes,
  ...flex.propTypes,
  ...alignSelf.propTypes,
  ...justifySelf.propTypes,
  ...borders.propTypes,
  ...overflow.propTypes,
};

export default Box;
