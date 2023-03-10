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

import { Text } from 'design';

import { ListItem } from 'teleterm/ui/components/ListItem';

export function NavigationItem({ item, closeMenu }: NavigationItemProps) {
  const handleClick = () => {
    item.onNavigate();
    closeMenu();
  };

  return (
    <StyledListItem as="button" type="button" onClick={handleClick}>
      <item.Icon fontSize={2} />
      <Text>{item.title}</Text>
    </StyledListItem>
  );
}

const StyledListItem = styled(ListItem)`
  height: 38px;
  gap: 12px;
  padding: 0 12px;
  border-radius: 0;
`;

type NavigationItemProps = {
  item: {
    title: string;
    Icon: React.ComponentType<{ fontSize: number }>;
    onNavigate: () => void;
  };
  closeMenu: () => void;
};
