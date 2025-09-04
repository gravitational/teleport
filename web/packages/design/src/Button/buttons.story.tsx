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

import { Fragment } from 'react';
import styled from 'styled-components';

import { Box, Input, Link, MenuItem, P1 } from 'design';

import {
  Button,
  ButtonBorder,
  ButtonFill,
  ButtonPrimary,
  ButtonPrimaryBorder,
  ButtonProps,
  ButtonSecondary,
  ButtonText,
  ButtonWarning,
} from '.';
import ButtonIcon from '../ButtonIcon';
import ButtonLink from '../ButtonLink';
import { ButtonWithMenu } from '../ButtonWithMenu';
import Flex, { Stack } from '../Flex';
import * as icons from '../Icon';

export default {
  title: 'Design/Button',
};

export const Buttons = () => {
  const fills: ButtonFill[] = ['filled', 'minimal', 'border'];
  return (
    <Flex gap={5} flexDirection="column" alignItems="flex-start">
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
                <ButtonTableCells fill={fill} intent="neutral" />
              </tr>
              <tr>
                <th>primary</th>
                <ButtonTableCells fill={fill} intent="primary" />
              </tr>
              <tr>
                <th>danger</th>
                <ButtonTableCells fill={fill} intent="danger" />
              </tr>
              <tr>
                <th>success</th>
                <ButtonTableCells fill={fill} intent="success" />
              </tr>
            </Fragment>
          ))}
        </tbody>
      </Table>{' '}
      <Flex gap={3}>
        <ButtonPrimary>Primary</ButtonPrimary>
        <ButtonPrimaryBorder>Primary (border)</ButtonPrimaryBorder>
        <ButtonSecondary>Secondary</ButtonSecondary>
        <ButtonBorder>Border</ButtonBorder>
        <ButtonWarning>Warning</ButtonWarning>
      </Flex>
      <Flex gap={3} alignItems="center">
        <Button size="extra-large">Extra large</Button>
        <Button size="large">Large</Button>
        <Button size="medium">Medium</Button>
        <Button size="small">Small</Button>
      </Flex>
      <Flex flexDirection="column" gap={3} alignItems="flex-start">
        <Input
          defaultValue="Padding of buttons below should match padding of this input"
          width="480px"
        />
        <Button size="extra-large" inputAlignment>
          Extra large with input alignment
        </Button>
        <Button size="large" inputAlignment>
          Large with input alignment
        </Button>
        <Button size="medium" inputAlignment>
          Medium with input alignment
        </Button>
        <Button size="small" inputAlignment>
          Small with input alignment
        </Button>
      </Flex>
      <Button block>block = true</Button>
      <Flex gap={3}>
        <Button disabled>Disabled</Button>
        <Box className="teleport-button__force-focus-visible">
          <Button>Focused</Button>
        </Box>
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
          text="Small"
          size="small"
          onClick={() => alert('Small button with menu')}
        >
          {menuItemsForButtonWithMenu}
        </ButtonWithMenu>
        <ButtonWithMenu
          text="Medium"
          size="medium"
          onClick={() => alert('Medium button with menu')}
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
          text="Extra large"
          size="extra-large"
          onClick={() => alert('Extra large button with menu')}
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
        <Button as="a" href="https://example.com" target="_blank" disabled>
          Link as button, disabled
        </Button>
        <ButtonIcon size={1} as="a" href="https://example.com" target="_blank">
          <icons.Link />
        </ButtonIcon>
      </Flex>
      <Flex gap={3}>
        <ButtonLink href="">Button Link</ButtonLink>
        <ButtonText>Button Text</ButtonText>
      </Flex>
      <Flex gap={3} flexDirection="column" alignItems="flex-start">
        {([2, 1, 0] as const).map(size => (
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
    </Flex>
  );
};

const Table = styled.table`
  border-collapse: collapse;

  th,
  td {
    border: ${p => p.theme.borders[1]};
    padding: 10px;
  }
`;

const ButtonTableCells = (props: ButtonProps<'button'>) => (
  <>
    {[
      '',
      'teleport-button__force-hover',
      'teleport-button__force-active',
      'teleport-button__force-focus-visible',
    ].map(className => (
      <td key={className} className={className}>
        <Button {...props}>Teleport</Button>
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

export const MinWidthFitContent = () => (
  <Stack gap={5}>
    <Link
      target="_blank"
      href="https://html-css-tip-of-the-week.netlify.app/tip/max-width-fit-content/"
    >
      https://html-css-tip-of-the-week.netlify.app/tip/max-width-fit-content/
    </Link>

    <P1>
      By default, a button text wraps itself at the first chance it gets,{' '}
      <Emphasis>even when it doesn't help avoid overflow</Emphasis>.
    </P1>

    <Stack gap={3}>
      <Flex gap={2} width="200px" p={2} border={1} borderColor="red">
        <Button>Lorem Ipsum</Button>
        <Button>Dolor Sit Amet</Button>
      </Flex>
      <Flex gap={2} width="120px" border={1} p={2} flexWrap="wrap">
        <Button>Lorem Ipsum</Button>
        <Button>Dolor Sit Amet</Button>
      </Flex>
    </Stack>

    <P1>
      With <code>text-wrap: nowrap</code>, the button text doesn't ever wrap,
      meaning that at narrow widths it introduces overflow{' '}
      <Emphasis>even when it could be avoided</Emphasis>.
    </P1>
    <Stack
      gap={3}
      css={`
        & button {
          text-wrap: nowrap;
        }
      `}
    >
      <Flex gap={2} width="200px" border={1} p={2}>
        <Button>Lorem Ipsum</Button>
        <Button>Dolor Sit Amet</Button>
      </Flex>
      <Flex
        gap={2}
        width="120px"
        p={2}
        flexWrap="wrap"
        border={1}
        borderColor="red"
      >
        <Button>Lorem Ipsum</Button>
        <Button>Dolor Sit Amet</Button>
      </Flex>
    </Stack>

    <P1>
      With <code>min-width: fit-content</code>, the button text is allowed to
      wrap as a last resort.
    </P1>
    <Flex gap={2} width="200px" border={1} p={2}>
      <Button minWidth="fit-content">Lorem Ipsum</Button>
      <Button minWidth="fit-content">Dolor Sit Amet</Button>
    </Flex>
    <Flex gap={2} width="120px" border={1} p={2} flexWrap="wrap">
      <Button minWidth="fit-content">Lorem Ipsum</Button>
      <Button minWidth="fit-content">Dolor Sit Amet</Button>
    </Flex>
  </Stack>
);

const Emphasis = styled.span`
  text-decoration-line: underline;
  text-decoration-color: red;
`;
