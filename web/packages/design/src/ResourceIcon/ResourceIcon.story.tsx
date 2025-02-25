/**
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

import React, { PropsWithChildren } from 'react';
import { useTheme } from 'styled-components';

import { Flex, Text } from 'design';
import { ResourceIcon } from 'design/ResourceIcon';

import { iconNames } from './resourceIconSpecs';

export default {
  title: 'Design/ResourceIcon',
};

export const Icons = () => {
  return (
    <Flex flexWrap="wrap">
      {iconNames.map(icon => {
        return (
          <IconBox text={icon} key={icon}>
            <ResourceIcon name={icon} width="100px" />
            <ResourceIcon name={icon} width="25px" />
          </IconBox>
        );
      })}
    </Flex>
  );
};

const IconBox: React.FC<PropsWithChildren<{ text: string }>> = ({
  children,
  text,
}) => {
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
      <Flex justifyContent="center" p={2} gap={2}>
        {children}
      </Flex>
      <Text typography="body2" mt={2}>
        {text}
      </Text>
    </Flex>
  );
};
