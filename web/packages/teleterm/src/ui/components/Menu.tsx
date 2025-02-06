/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { ElementType } from 'react';
import styled from 'styled-components';

import { Flex, Text } from 'design';

import { KeyboardShortcutAction } from 'teleterm/services/config';
import { useKeyboardShortcutFormatters } from 'teleterm/ui/services/keyboardShortcuts';

import { ListItem } from './ListItem';

export type MenuItem = {
  title: string;
  Icon?: ElementType;
  onNavigate?(): void;
  prependSeparator?: boolean;
  keyboardShortcutAction?: KeyboardShortcutAction;
} & (MenuItemAlwaysEnabled | MenuItemConditionallyDisabled);

type MenuItemAlwaysEnabled = { isDisabled?: false };
type MenuItemConditionallyDisabled = { isDisabled: true; disabledText: string };

export const Menu = styled.menu`
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  min-width: 280px;
  background: ${props => props.theme.colors.levels.elevated};
`;

export function MenuListItem({
  item,
  closeMenu,
}: {
  item: MenuItem;
  closeMenu(): void;
}) {
  const { getAccelerator } = useKeyboardShortcutFormatters();
  const handleClick = () => {
    item.onNavigate?.();
    closeMenu();
  };

  return (
    <>
      {item.prependSeparator && <Separator />}
      <MenuItemContainer
        as="button"
        type="button"
        disabled={item.isDisabled}
        title={item.isDisabled ? item.disabledText : undefined}
        onClick={handleClick}
      >
        {item.Icon && (
          <item.Icon
            color={item.isDisabled ? 'text.disabled' : null}
            size="medium"
          />
        )}
        <Flex
          gap={2}
          flex="1"
          alignItems="baseline"
          justifyContent="space-between"
        >
          <Text>{item.title}</Text>

          {item.keyboardShortcutAction && (
            <Text
              fontSize={1}
              css={`
                border-radius: 4px;
                width: fit-content;
                // Using a background with an alpha color to make this interact better with the
                // disabled state.
                background-color: ${props =>
                  props.theme.colors.spotBackground[0]};
                padding: ${props => props.theme.space[1]}px
                  ${props => props.theme.space[1]}px;
              `}
            >
              {getAccelerator(item.keyboardShortcutAction)}
            </Text>
          )}
        </Flex>
      </MenuItemContainer>
    </>
  );
}

export const Separator = styled.div`
  background: ${props => props.theme.colors.spotBackground[1]};
  height: 1px;
`;

export const MenuItemContainer = styled(ListItem)<{
  disabled: boolean;
}>`
  min-height: 38px;
  height: auto;
  gap: ${props => props.theme.space[3]}px;
  padding: 0 ${props => props.theme.space[3]}px;
  border-radius: 0;

  &:disabled {
    cursor: default;
    color: ${props => props.theme.colors.text.disabled};

    &:hover {
      background-color: inherit;
    }
  }
`;
