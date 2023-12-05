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

import { AlignItemsProps, FlexDirectionProps, FlexWrapProps, JustifyContentProps } from 'styled-system';

import { alignItems, flexDirection, flexWrap, gap, justifyContent } from 'design/system';

import { BoxProps } from 'design/Box/Box';
import { GapProps } from 'design/system/gap';

import Box from '../Box';

export type FlexProps = AlignItemsProps &
  JustifyContentProps &
  FlexWrapProps &
  FlexDirectionProps &
  GapProps &
  BoxProps;

const Flex = styled(Box)<FlexProps>`
  display: flex;
  ${alignItems}
  ${justifyContent}
  ${flexWrap}
  ${flexDirection}
  ${gap};
`;

Flex.displayName = 'Flex';

export default Flex;
