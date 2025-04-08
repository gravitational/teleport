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

import PropTypes from 'prop-types';
import React from 'react';
import styled from 'styled-components';

import {
  alignSelf,
  color,
  height,
  maxHeight,
  maxWidth,
  space,
  width,
} from 'design/system';

const Image = props => {
  return <StyledImg {...props} />;
};

Image.propTypes = {
  /** Image Src */
  src: PropTypes.string,
  ...space.propTypes,
  ...color.propTypes,
  ...width.propTypes,
  ...height.propTypes,
  ...maxWidth.propTypes,
  ...maxHeight.propTypes,
};

Image.displayName = 'Logo';

export default Image;

const StyledImg = styled.img`
  display: block;
  outline: none;
  ${color} ${space} ${width} ${height} ${maxWidth} ${maxHeight} ${alignSelf}
`;
