/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import styled from 'styled-components';

import { Box, Text, Flex } from 'design';

export function NavigationItem({ item, closeMenu }: NavigationItemProps) {
  const handleClick = () => {
    item.onNavigate();
    closeMenu();
  };

  return (
    <ListItem p={2} pl={0} gap={2} onClick={handleClick}>
      <IconBox p={2}>{item.Icon}</IconBox>
      <Text>{item.title}</Text>
    </ListItem>
  );
}

const ListItem = styled(Flex)`
  border-radius: 4px;
  align-items: center;
  justify-content: start;
  cursor: pointer;
  &:hover {
    background: ${props => props.theme.colors.levels.elevated};
  }
`;

const IconBox = styled(Box)`
  display: flex;
  align-items: center;
  border-radius: 4px;
  background-color: ${props => props.theme.colors.levels.elevated};
`;

type NavigationItemProps = {
  item: {
    title: string;
    Icon: JSX.Element;
    onNavigate: () => void;
  };
  closeMenu: () => void;
};
