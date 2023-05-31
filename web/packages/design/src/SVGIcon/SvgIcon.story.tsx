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

import { useTheme } from 'styled-components';

import { IconCircle } from 'design/Icon/IconCircle';

import Flex from '../Flex';
import Text from '../Text';

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
        const S = SvgIcons[icon];
        const size = 64;

        return (
          <>
            <IconBox text={icon}>
              <IconContainer Icon={S} size={size} />
            </IconBox>
            <IconBox text={icon}>
              <IconCircle Icon={S} size={size} />
            </IconBox>
          </>
        );
      })}
    </Flex>
  );
};

const IconBox = ({ children, text }) => {
  const theme = useTheme();

  return (
    <Flex
      width="150px"
      height="150px"
      alignItems="center"
      justifyContent="center"
      bg={theme.colors.spotBackground[0]}
      flexDirection="column"
      m={1}
    >
      <Flex justifyContent="center" p={2}>
        {children}
      </Flex>
      <Text typography="paragraph2" mt={2}>
        {text}
      </Text>
    </Flex>
  );
};

const IconContainer = ({ Icon, size }) => {
  const theme = useTheme();

  return (
    <Icon
      size={size}
      bg={theme.colors.spotBackground[0]}
      fill={theme.colors.text.main}
    />
  );
};
