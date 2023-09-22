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

import React, { useState } from 'react';
import { ButtonIcon, Flex, Label, Text } from 'design';
import { ExitRight } from 'design/Icon';

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
        <Text typography="body1" title={userWithClusterName}>
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
            <ExitRight ml="2px" fontSize={14} />
          </ButtonIcon>
        </Flex>
      </Flex>
    </ListItem>
  );
}
