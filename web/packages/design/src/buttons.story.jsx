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

import ButtonLink from './ButtonLink';
import ButtonIcon from './ButtonIcon';
import { AddUsers, Trash, Ellipsis } from './Icon';
import Flex from './Flex';
import Button, {
  ButtonPrimary,
  ButtonSecondary,
  ButtonWarning,
  ButtonBorder,
  ButtonText,
} from './Button';

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
        <AddUsers />
        Add users
      </ButtonPrimary>
    </Flex>

    <Flex gap={3}>
      <Button as="a" href="https://example.com" target="_blank">
        Link as button
      </Button>
      <ButtonSecondary as="a" href="https://example.com" target="_blank">
        Link as button
      </ButtonSecondary>
    </Flex>

    <Flex gap={3}>
      <ButtonLink href="">Button Link</ButtonLink>
      <ButtonText>Button Text</ButtonText>
    </Flex>

    <Flex gap={3}>
      <ButtonIcon size={2}>
        <AddUsers />
      </ButtonIcon>
      <ButtonIcon size={2}>
        <Ellipsis />
      </ButtonIcon>
      <ButtonIcon size={2}>
        <Trash />
      </ButtonIcon>
    </Flex>

    <Flex gap={3}>
      <ButtonIcon size={1}>
        <AddUsers />
      </ButtonIcon>
      <ButtonIcon size={1}>
        <Ellipsis />
      </ButtonIcon>
      <ButtonIcon size={1}>
        <Trash />
      </ButtonIcon>
    </Flex>

    <Flex gap={3}>
      <ButtonIcon size={0}>
        <AddUsers />
      </ButtonIcon>
      <ButtonIcon size={0}>
        <Ellipsis />
      </ButtonIcon>
      <ButtonIcon size={0}>
        <Trash />
      </ButtonIcon>
    </Flex>
  </Flex>
);
