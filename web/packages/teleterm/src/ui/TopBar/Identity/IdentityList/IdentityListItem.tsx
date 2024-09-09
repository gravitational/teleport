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

import React, { useState } from 'react';
import { ButtonIcon, Flex, Label, Text } from 'design';
import { Logout } from 'design/Icon';

import { ListItem } from 'teleterm/ui/components/ListItem';
import { useKeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation';
import { getUserWithClusterName } from 'teleterm/ui/utils';

interface IdentityListItemProps {
  index: number;
  userName?: string;
  clusterName: string;
  isSelected: boolean;

  onSelect(): void;
  onLogout(): void;
}

export function IdentityListItem(props: IdentityListItemProps) {
  const [isHovered, setIsHovered] = useState(false);
  const { isActive } = useKeyboardArrowsNavigation({
    index: props.index,
    onRun: props.onSelect,
  });

  const userWithClusterName = getUserWithClusterName(props);

  return (
    <ListItem
      css={`
        border-radius: 0;
        height: 38px;
      `}
      onClick={props.onSelect}
      isActive={isActive}
      onMouseEnter={() => {
        setIsHovered(true);
      }}
      onMouseLeave={() => {
        setIsHovered(false);
      }}
    >
      <Flex justifyContent="space-between" alignItems="center" width="100%">
        <Text typography="body2" title={userWithClusterName}>
          {userWithClusterName}
        </Text>
        <Flex alignItems="center">
          {props.isSelected ? (
            <Label kind="success" ml={2} style={{ height: 'fit-content' }}>
              active
            </Label>
          ) : null}
          <ButtonIcon
            mr="-10px"
            style={{
              visibility: isHovered ? 'visible' : 'hidden',
              transition: 'none',
            }}
            ml={2}
            title={`Log out from ${props.clusterName}`}
            onClick={e => {
              e.stopPropagation();
              props.onLogout();
            }}
          >
            {/* Due to the icon shape it appears to be not centered, so a small margin is added */}
            <Logout ml="2px" size="small" />
          </ButtonIcon>
        </Flex>
      </Flex>
    </ListItem>
  );
}
