/*
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

import { PropsWithChildren, useRef, useState } from 'react';

import Box from '../Box';
import { ButtonPrimary } from '../Button';
import Flex from '../Flex';
import * as Icons from '../Icon';
import { Origin } from '../Popover';
import { H3 } from '../Text';
import Menu from './Menu';
import MenuItem, {
  MenuItemSectionLabel,
  MenuItemSectionSeparator,
} from './MenuItem';
import MenuItemIcon from './MenuItemIcon';
import MenuList from './MenuList';

export default {
  title: 'Design/Menu',
};

export const PlacementExample = () => (
  <Flex m={3} gap={8} flexWrap="wrap">
    <SimpleMenu text="Menu to right">
      <MenuItem>Lorem</MenuItem>
      <MenuItem>Ipsum</MenuItem>
      <MenuItem>Dolor</MenuItem>
      <MenuItem>Sit</MenuItem>
      <MenuItem>Amet</MenuItem>
    </SimpleMenu>
    <SimpleMenu
      text="Menu in center"
      anchorOrigin={{
        vertical: 'bottom',
        horizontal: 'center',
      }}
      transformOrigin={{
        vertical: 'top',
        horizontal: 'center',
      }}
    >
      <MenuItem>Test</MenuItem>
      <MenuItem>Test2</MenuItem>
      <MenuItem>
        <ButtonPrimary mt={2} mb={2} block>
          Logout
        </ButtonPrimary>
      </MenuItem>
    </SimpleMenu>
    <SimpleMenu
      text="Menu to left"
      anchorOrigin={{
        vertical: 'top',
        horizontal: 'right',
      }}
      transformOrigin={{
        vertical: 'top',
        horizontal: 'right',
      }}
    >
      <MenuItem>Test</MenuItem>
      <MenuItem>Test2</MenuItem>
    </SimpleMenu>
  </Flex>
);

export const MenuItems = () => (
  <Flex m={3} gap={8}>
    <Flex gap={3} flexDirection="column">
      <H3>Label after separator</H3>
      <OpenMenu>
        <MenuItem>Lorem ipsum</MenuItem>
        <MenuItem>Dolor sit amet</MenuItem>
        <MenuItemSectionSeparator />
        <MenuItemSectionLabel>Leo vitae arcu</MenuItemSectionLabel>
        <MenuItem>Donec volutpat</MenuItem>
        <MenuItem>Mauris sit</MenuItem>
        <MenuItem>Amet nisi tempor</MenuItem>
      </OpenMenu>
    </Flex>
    <Flex gap={3} flexDirection="column">
      <H3>Menu item after separator</H3>
      <OpenMenu>
        <MenuItem>Lorem ipsum</MenuItem>
        <MenuItem>Dolor sit amet</MenuItem>
        <MenuItemSectionSeparator />
        <MenuItem>Leo vitae arcu</MenuItem>
        <MenuItem>Donec volutpat</MenuItem>
        <MenuItem>Mauris sit</MenuItem>
        <MenuItem>Amet nisi tempor</MenuItem>
      </OpenMenu>
    </Flex>
    <Flex gap={3} flexDirection="column">
      <H3>Label as first child</H3>
      <OpenMenu>
        <MenuItemSectionLabel>Tempus ut libero</MenuItemSectionLabel>
        <MenuItem>Lorem ipsum</MenuItem>
        <MenuItem>Dolor sit amet</MenuItem>
        <MenuItemSectionSeparator />
        <MenuItemSectionLabel>Leo vitae arcu</MenuItemSectionLabel>
        <MenuItem>Donec volutpat</MenuItem>
        <MenuItem>Mauris sit</MenuItem>
      </OpenMenu>
    </Flex>
  </Flex>
);

export const IconExample = () => (
  <Menu
    anchorOrigin={{
      vertical: 'bottom',
      horizontal: 'center',
    }}
    transformOrigin={{
      vertical: 'top',
      horizontal: 'center',
    }}
    open={true}
  >
    <MenuItem data-testid="item">
      <MenuItemIcon data-testid="icon" as={Icons.Apple} />
      Test
    </MenuItem>
    <MenuItem data-testid="item">
      <MenuItemIcon data-testid="icon" as={Icons.Cash} />
      Test
    </MenuItem>
    <MenuItem data-testid="item">
      <MenuItemIcon data-testid="icon" as={Icons.CircleArrowLeft} />
      Test
    </MenuItem>
  </Menu>
);

const SimpleMenu = (
  props: PropsWithChildren<{
    text: string;
    anchorOrigin?: Origin;
    transformOrigin?: Origin;
  }>
) => {
  const anchorElRef = useRef(null);
  const [isOpen, setIsOpen] = useState(false);

  const open = () => {
    setIsOpen(true);
  };
  const close = () => {
    setIsOpen(false);
  };

  const { text, anchorOrigin, transformOrigin, children } = props;

  return (
    <Box textAlign="center">
      <ButtonPrimary size="small" setRef={anchorElRef} onClick={open}>
        {text}
      </ButtonPrimary>
      <Menu
        anchorOrigin={anchorOrigin}
        transformOrigin={transformOrigin}
        anchorEl={anchorElRef.current}
        open={isOpen}
        onClose={close}
      >
        {children}
      </Menu>
    </Box>
  );
};

const OpenMenu = (props: PropsWithChildren) => {
  // MenuList uses a percent value in max-height combined with overflow: hidden, so we have
  // to wrap it twice here to avoid issues with the menu being cut off.
  return (
    <Box>
      <Box>
        <MenuList>{props.children}</MenuList>
      </Box>
    </Box>
  );
};
