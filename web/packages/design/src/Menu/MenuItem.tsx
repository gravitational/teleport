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

import styled from 'styled-components';
import {
  color,
  ColorProps,
  fontSize,
  FontSizeProps,
  space,
  SpaceProps,
} from 'styled-system';

import Flex from 'design/Flex';
import { Theme } from 'design/theme/themes/types';

const defaultValues = {
  fontSize: 1,
  px: 3,
};

interface MenuItemProps extends FontSizeProps, SpaceProps, ColorProps {
  disabled?: boolean;
}

interface ThemedMenuItemProps extends MenuItemProps {
  theme: Theme;
}

// TODO(ravicious): This probably can be simplified when the time comes to do a redesign of Menu.
// For now it's based on the existing code from fromTheme so that we don't break anything.
const fromThemeBase = (props: { theme: Theme }) => {
  const values = {
    ...defaultValues,
    ...props,
  };
  return {
    ...fontSize(values),
    ...space(values),
    ...color(values),
    fontWeight: values.theme.regular,
  };
};

const MenuItemBase = styled(Flex)`
  min-height: 40px;
  box-sizing: border-box;
  justify-content: flex-start;
  align-items: center;
  min-width: 140px;
  overflow: hidden;
  text-decoration: none;
  white-space: nowrap;
  color: ${props => props.theme.colors.text.main};

  ${fromThemeBase}
`;

export const MenuItemSectionSeparator = styled.hr.attrs({
  onClick: event => {
    // Make sure that clicks on this element don't trigger onClick set on MenuList.
    event.stopPropagation();
  },
})`
  background: ${props => props.theme.colors.interactive.tonal.neutral[1]};
  height: 1px;
  border: 0;
  font-size: 0;
`;

export const MenuItemSectionLabel = styled(MenuItemBase).attrs({
  px: 2,
  onClick: event => {
    // Make sure that clicks on this element don't trigger onClick set on MenuList.
    event.stopPropagation();
  },
})`
  font-weight: bold;
  min-height: 16px;

  // Add padding to the label for extra visual space, but only when it follows a separator or is the
  // first child.
  //
  // If a separator follows a MenuItem, there's already enough visual space between MenuItem and
  // separator, so no extra space is needed. The hover state of MenuItem highlights everything right
  // from the separator start to the end of MenuItem.
  //
  // Padding is used instead of margin here on purpose, so that there's no empty transparent space
  // between Separator and Label – otherwise clicking on that space would count as a click on
  // MenuList and not trigger onClick set on Separator or Label.
  ${MenuItemSectionSeparator} + &, &:first-child {
    padding-top: ${props => props.theme.space[1]}px;
  }
`;

const fromTheme = (props: ThemedMenuItemProps) => {
  const values = {
    ...defaultValues,
    ...props,
  };
  return {
    ...fontSize(values),
    ...space(values),
    ...color(values),
    fontWeight: values.theme.regular,

    '&:hover, &:focus': {
      color: props.disabled
        ? values.theme.colors.text.disabled
        : values.theme.colors.text.main,
      background: values.theme.colors.spotBackground[0],
    },
    '&:active': {
      background: values.theme.colors.spotBackground[1],
    },
  };
};

const MenuItem = styled(MenuItemBase).attrs({
  role: 'menuitem',
})<MenuItemProps>`
  cursor: ${props => (props.disabled ? 'not-allowed' : 'pointer')};
  color: ${props =>
    props.disabled
      ? props.theme.colors.text.disabled
      : props.theme.colors.text.main};

  ${fromTheme}
`;

MenuItem.displayName = 'MenuItem';

export default MenuItem;
