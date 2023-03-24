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

import {
  alignItems,
  alignSelf,
  border,
  borderColor,
  borders,
  color,
  flex,
  flexDirection,
  flexWrap,
  fontSize,
  fontWeight,
  height,
  justifyContent,
  justifySelf,
  maxHeight,
  maxWidth,
  minHeight,
  minWidth,
  overflow,
  propTypes,
  size,
  space,
  textAlign,
  width,
  style,
} from 'styled-system';

import typography from './typography';
import borderRadius from './borderRadius';

const gap = style({
  prop: 'gap',
  cssProperty: 'gap',
  // This makes gap use the space defined in the theme.
  // https://github.com/styled-system/styled-system/blob/v3.1.11/src/index.js#L67
  key: 'space',
});

propTypes.gap = gap.propTypes;

export {
  alignItems,
  alignSelf,
  border,
  borderColor,
  borders,
  borderRadius,
  color,
  flex,
  flexDirection,
  flexWrap,
  fontSize,
  fontWeight,
  gap,
  height,
  justifyContent,
  justifySelf,
  maxHeight,
  maxWidth,
  minHeight,
  minWidth,
  overflow,
  propTypes,
  size,
  space,
  textAlign,
  typography,
  width,
};
