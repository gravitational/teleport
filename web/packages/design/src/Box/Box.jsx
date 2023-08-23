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

import styled from 'styled-components';

import {
  overflow,
  borders,
  borderRadius,
  borderColor,
  flex,
  height,
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
