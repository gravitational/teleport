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
  alignItems,
  justifyContent,
  flexWrap,
  flexDirection,
  gap,
  propTypes,
} from 'design/system';

import Box from '../Box';

const Flex = styled(Box)`
  display: flex;
  ${alignItems}
  ${justifyContent}
  ${flexWrap}
  ${flexDirection}
  ${gap};
`;

Flex.propTypes = {
  ...propTypes.Box,
  ...propTypes.alignItems,
  ...propTypes.justifyContent,
  ...propTypes.flexWrap,
  ...propTypes.flexDirection,
  ...propTypes.gap,
};

Flex.displayName = 'Flex';

export default Flex;
