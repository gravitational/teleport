/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import styled from 'styled-components';
import PropTypes from 'prop-types';

import {
  space,
  color,
  width,
  height,
  maxWidth,
  maxHeight,
  alignSelf,
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
