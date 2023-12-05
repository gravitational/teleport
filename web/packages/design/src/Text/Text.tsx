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

import { ColorProps, FontSizeProps, FontWeightProps, SpaceProps, TextAlignProps } from 'styled-system';

import { color, fontSize, fontWeight, space, textAlign, typography } from 'design/system';
import { TypographyProps } from 'design/system/typography';

export type TextProps = TypographyProps &
  FontSizeProps &
  SpaceProps &
  ColorProps &
  TextAlignProps &
  FontWeightProps;

const Text = styled.div<TextProps>`
  overflow: hidden;
  text-overflow: ellipsis;
  ${typography}
  ${fontSize}
  ${space}
  ${color}
  ${textAlign}
  ${fontWeight}
`;

Text.displayName = 'Text';

Text.defaultProps = {
  m: 0,
};

export default Text;
