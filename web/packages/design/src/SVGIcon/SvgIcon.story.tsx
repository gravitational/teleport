/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
import React from 'react';

import Flex from '../Flex';
import Box from '../Box';

import * as SvgIcons from '.';

export default {
  title: 'Design/Icon',
};

export const PreferSvgIcons = () => {
  const icons = Object.keys(SvgIcons);
  return (
    <Flex flexWrap="wrap">
      {icons.map(icon => {
        // eslint-disable-next-line import/namespace
        return <IconBox Icon={SvgIcons[icon]} text={icon} />;
      })}
    </Flex>
  );
};

const IconBox = ({ Icon, text }) => (
  <Flex m={3} width="300px">
    <Box width="40px" textAlign="center">
      <Icon />
    </Box>
    <Box ml={2}>{text}</Box>
  </Flex>
);
