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

import { Button, Flex, IconButton, type ButtonProps } from '@chakra-ui/react';
import styled from '@emotion/styled';
import type { Meta, StoryObj } from '@storybook/react';
import { Fragment } from 'react';

import {
  ButtonBorder,
  ButtonPrimary,
  ButtonSecondary,
  ButtonWarning,
} from '../components/button';
import { Input } from '../components/input/Input';

const meta: Meta<typeof Button> = {
  component: Button,
  title: 'Chakra UI/Button',
};

export default meta;

type Story = StoryObj<typeof Button>;

export const Buttons = () => {
  const fills: ButtonProps['variant'][] = ['filled', 'minimal', 'border'];

  return (
    <Flex gap={5} direction="column" align="flex-start">
      <Table>
        <thead>
          <tr>
            <th colSpan={2} rowSpan={2} />
            <th colSpan={4}>Enabled</th>
            <th colSpan={2}>Disabled</th>
          </tr>
          <tr>
            <th>Default</th>
            <th>Hover</th>
            <th>Active</th>
            <th>Focus</th>
            <th>Default</th>
            <th>Hover</th>
          </tr>
        </thead>
        <tbody>
          {fills.map(fill => (
            <Fragment key={fill}>
              <tr>
                <th rowSpan={4}>{fill}</th>
                <th>neutral</th>
                <ButtonTableCells variant={fill} intent="neutral" />
              </tr>
              <tr>
                <th>primary</th>
                <ButtonTableCells variant={fill} intent="primary" />
              </tr>
              <tr>
                <th>danger</th>
                <ButtonTableCells variant={fill} intent="danger" />
              </tr>
              <tr>
                <th>success</th>
                <ButtonTableCells variant={fill} intent="success" />
              </tr>
            </Fragment>
          ))}
        </tbody>
      </Table>{' '}
      <Flex gap={3}>
        <ButtonPrimary>Primary</ButtonPrimary>
        <ButtonSecondary>Secondary</ButtonSecondary>
        <ButtonBorder>Border</ButtonBorder>
        <ButtonWarning>Warning</ButtonWarning>
      </Flex>
      <Flex gap={3} alignItems="center">
        <Button size="xl">Extra large</Button>
        <Button size="xl" intent="primary" variant="minimal">
          Extra large
        </Button>
        <Button size="lg">Large</Button>
        <Button size="md">Medium</Button>
        <Button size="sm">Small</Button>
      </Flex>
      <Flex flexDirection="column" gap={3} alignItems="flex-start">
        <Input
          defaultValue="Padding of buttons below should match padding of this input"
          width="480px"
        />
        <Button size="xl" inputAlignment>
          Extra large with input alignment
        </Button>
        <Button size="lg" inputAlignment>
          Large with input alignment
        </Button>
        <Button size="md" inputAlignment>
          Medium with input alignment
        </Button>
        <Button size="sm" inputAlignment>
          Small with input alignment
        </Button>
      </Flex>
      <Button block>block = true</Button>
      <Flex gap={3}>
        <Button disabled>Disabled</Button>
        <Button autoFocus className="focus">
          Focused
        </Button>
      </Flex>
      <Flex gap={3}>
        <ButtonPrimary gap={2}>Add users</ButtonPrimary>
      </Flex>
      <Flex gap={3} alignItems="center">
        {/*<ButtonWithMenu*/}
        {/*  text="Button with menu"*/}
        {/*  onClick={() => alert('Button with menu')}*/}
        {/*>*/}
        {/*  {menuItemsForButtonWithMenu}*/}
        {/*</ButtonWithMenu>*/}
        {/*<ButtonWithMenu*/}
        {/*  text="Small"*/}
        {/*  size="small"*/}
        {/*  onClick={() => alert('Small button with menu')}*/}
        {/*>*/}
        {/*  {menuItemsForButtonWithMenu}*/}
        {/*</ButtonWithMenu>*/}
        {/*<ButtonWithMenu*/}
        {/*  text="Medium"*/}
        {/*  size="medium"*/}
        {/*  onClick={() => alert('Medium button with menu')}*/}
        {/*>*/}
        {/*  {menuItemsForButtonWithMenu}*/}
        {/*</ButtonWithMenu>*/}
        {/*<ButtonWithMenu*/}
        {/*  text="Large"*/}
        {/*  size="large"*/}
        {/*  onClick={() => alert('Large button with menu')}*/}
        {/*>*/}
        {/*  {menuItemsForButtonWithMenu}*/}
        {/*</ButtonWithMenu>*/}
        {/*<ButtonWithMenu*/}
        {/*  text="Extra large"*/}
        {/*  size="extra-large"*/}
        {/*  onClick={() => alert('Extra large button with menu')}*/}
        {/*>*/}
        {/*  {menuItemsForButtonWithMenu}*/}
        {/*</ButtonWithMenu>*/}
        {/*<ButtonWithMenu text="With different icon" MenuIcon={icons.Cog}>*/}
        {/*  {menuItemsForButtonWithMenu}*/}
        {/*</ButtonWithMenu>*/}
      </Flex>
      <Flex gap={3}>
        <Button asChild>
          <a href="https://example.com" target="_blank" rel="noreferrer">
            Link as button
          </a>
        </Button>
        <ButtonSecondary asChild>
          <a href="https://example.com" target="_blank">
            Link as button
          </a>
        </ButtonSecondary>
        <Button asChild disabled>
          <a href="https://example.com" target="_blank">
            Link as button, disabled
          </a>
        </Button>
        <IconButton
          size={1}
          as="a"
          href="https://example.com"
          target="_blank"
        ></IconButton>
      </Flex>
      <Flex gap={3}>
        {/*<ButtonLink href="">Button Link</ButtonLink>*/}
        {/*<ButtonText>Button Text</ButtonText>*/}
      </Flex>
      {/*<Flex gap={3} flexDirection="column" alignItems="flex-start">*/}
      {/*  {[2, 1, 0].map(size => (*/}
      {/*    <Flex gap={3} key={`size-${size}`}>*/}
      {/*      <ButtonIcon size={size}>*/}
      {/*        <icons.AddUsers />*/}
      {/*      </ButtonIcon>*/}
      {/*      <ButtonIcon size={size}>*/}
      {/*        <icons.Ellipsis />*/}
      {/*      </ButtonIcon>*/}
      {/*      <ButtonIcon size={size} disabled>*/}
      {/*        <icons.Trash />*/}
      {/*      </ButtonIcon>*/}
      {/*    </Flex>*/}
      {/*  ))}*/}
      {/*</Flex>*/}
    </Flex>
  );
};

Buttons.parameters = {
  pseudo: {
    hover: '.hover',
    focusVisible: '.focus',
    active: '.active',
  },
};

//
const Table = styled.table`
  border-collapse: collapse;

  th,
  td {
    border: 1px solid white;
    padding: 10px;
  }
`;

const ButtonTableCells = (props: ButtonProps) => (
  <>
    {['', 'hover', 'active', 'focus'].map(className => (
      <td key={className}>
        <Button {...props} className={className}>
          Teleport
        </Button>
      </td>
    ))}
    {['', 'teleport-button__force-hover'].map(className => (
      <td key={className} className={className}>
        <Button {...props} disabled>
          Teleport
        </Button>
      </td>
    ))}
  </>
);
//
// const menuItemsForButtonWithMenu = (
//   <>
//     <MenuItem onClick={() => alert('Foo')}>Foo</MenuItem>
//     <MenuItem as="a" href="https://example.com" target="_blank">
//       Link to example.com
//     </MenuItem>
//     <MenuItem onClick={() => alert('Lorem ipsum dolor sit amet')}>
//       Lorem ipsum dolor sit amet
//     </MenuItem>
//   </>
// );
