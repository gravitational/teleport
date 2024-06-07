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

import React from 'react';

import { MenuItem } from 'design';

import ButtonLink from './ButtonLink';
import ButtonIcon from './ButtonIcon';
import * as icons from './Icon';
import Flex from './Flex';
import Button, {
  ButtonPrimary,
  ButtonSecondary,
  ButtonWarning,
  ButtonBorder,
  ButtonText,
} from './Button';
import { ButtonWithMenu } from './ButtonWithMenu';

export default {
  title: 'Design/Button',
};

export const Buttons = () => (
  <Flex gap={4} flexDirection="column" alignItems="flex-start">
    <Flex gap={3}>
      <ButtonPrimary>Primary</ButtonPrimary>
      <ButtonSecondary>Secondary</ButtonSecondary>
      <ButtonBorder>Border</ButtonBorder>
      <ButtonWarning>Warning</ButtonWarning>
    </Flex>

    <Flex gap={3} alignItems="center">
      <Button size="large">Large</Button>
      <Button size="medium">Medium</Button>
      <Button size="small">Small</Button>
    </Flex>

    <Button block>block = true</Button>

    <Flex gap={3}>
      <Button disabled>Disabled</Button>
      <Button autoFocus>Focused</Button>
    </Flex>

    <Flex gap={3}>
      <ButtonPrimary gap={2}>
        <icons.AddUsers />
        Add users
      </ButtonPrimary>
    </Flex>

    <Flex gap={3} alignItems="center">
      <ButtonWithMenu
        text="Button with menu"
        onClick={() => alert('Button with menu')}
      >
        {menuItemsForButtonWithMenu}
      </ButtonWithMenu>
      <ButtonWithMenu
        text="Large"
        size="large"
        onClick={() => alert('Large button with menu')}
      >
        {menuItemsForButtonWithMenu}
      </ButtonWithMenu>
      <ButtonWithMenu
        text="Small"
        size="small"
        onClick={() => alert('Small button with menu')}
      >
        {menuItemsForButtonWithMenu}
      </ButtonWithMenu>
      <ButtonWithMenu text="With different icon" MenuIcon={icons.Cog}>
        {menuItemsForButtonWithMenu}
      </ButtonWithMenu>
    </Flex>

    <Flex gap={3}>
      <Button as="a" href="https://example.com" target="_blank">
        Link as button
      </Button>
      <ButtonSecondary as="a" href="https://example.com" target="_blank">
        Link as button
      </ButtonSecondary>
      <ButtonIcon size={1} as="a" href="https://example.com" target="_blank">
        <icons.Link />
      </ButtonIcon>
    </Flex>

    <Flex gap={3}>
      <ButtonLink href="">Button Link</ButtonLink>
      <ButtonText>Button Text</ButtonText>
    </Flex>

    {[2, 1, 0].map(size => (
      <Flex gap={3} key={`size-${size}`}>
        <ButtonIcon size={size}>
          <icons.AddUsers />
        </ButtonIcon>
        <ButtonIcon size={size}>
          <icons.Ellipsis />
        </ButtonIcon>
        <ButtonIcon size={size} disabled>
          <icons.Trash />
        </ButtonIcon>
      </Flex>
    ))}
  </Flex>
);

const menuItemsForButtonWithMenu = (
  <>
    <MenuItem onClick={() => alert('Foo')}>Foo</MenuItem>
    <MenuItem as="a" href="https://example.com" target="_blank">
      Link to example.com
    </MenuItem>
    <MenuItem onClick={() => alert('Lorem ipsum dolor sit amet')}>
      Lorem ipsum dolor sit amet
    </MenuItem>
  </>
);
