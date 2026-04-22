/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { useListItem, useMergeRefs } from '@floating-ui/react';
import React, { HTMLAttributes, forwardRef, useContext } from 'react';
import styled from 'styled-components';

import { Box, Flex, Text } from 'design';
import { CheckboxInput } from 'design/Checkbox';
import { DropdownMenuContext } from 'design/DropdownMenu/DropdownMenuContext';

const SECTION_PADDING_X = 3;

/**
 * The floating panel that contains menu content. Used automatically by
 * `DropdownMenu` as the default container. The panel automatically
 * shrinks to fit the viewport when space is limited.
 */
export const DropdownMenuPanel = styled(Box)`
  box-sizing: border-box;
  z-index: 1500;
  min-width: 150px;
  max-width: 300px;
  max-height: 400px;
  overflow-y: auto;
  overflow-x: clip;
  scrollbar-width: thin;
  scrollbar-color: ${p => p.theme.colors.spotBackground[2]} transparent;
  background: ${p => p.theme.colors.levels.elevated};
  border-radius: ${p => p.theme.radii[2]}px;
  box-shadow: ${p => p.theme.boxShadow[1]};
  /* Same-color border instead of padding so sticky sections
     sit against the panel edge without a visible gap. */
  border-block: ${p => p.theme.space[1]}px solid
    ${p => p.theme.colors.levels.elevated};
  outline: none;
`;

/**
 * A content section within a `DropdownMenu`. Wraps menu items with
 * consistent horizontal padding and vertical spacing between sections.
 *
 * Provide `header` to render a section header above the content.
 *
 * @example
 * ```tsx
 * <DropdownMenuSection header="Available Access">
 *   <DropdownMenuItem label="Production DB" onClick={handleConnect} />
 *   <DropdownMenuItem label="Staging DB" onClick={handleConnect} />
 * </DropdownMenuSection>
 * <DropdownMenuSection header="Request Access">
 *   <DropdownMenuCheckableItem label="Admin DB" checked={checked} onChange={toggle} />
 * </DropdownMenuSection>
 * ```
 */
export const DropdownMenuSection = ({
  header,
  children,
  ...bodyProps
}: {
  header?: React.ReactNode;
} & React.ComponentProps<typeof StyledBodySection>) => {
  return (
    <StyledBodySection {...bodyProps}>
      {!!header && <StyledHeader>{header}</StyledHeader>}
      {children}
    </StyledBodySection>
  );
};

const StyledBodySection = styled(Box).attrs({
  paddingX: SECTION_PADDING_X,
})`
  margin-top: ${({ theme }) => theme.space[1]}px;
  margin-bottom: ${({ theme }) => theme.space[2]}px;

  /* No top margin if first section or following another section */
  & + &,
  &:first-child {
    margin-top: 0;
  }
  /* No bottom margin if last section or before another section */
  &:has(+ &),
  &:last-child {
    margin-bottom: 0;
  }
  & > & {
    padding-left: 0;
    padding-right: 0;
  }
`;

/**
 * A sticky container that stays pinned to the top or bottom of the
 * scrollable menu panel.
 *
 * @example
 * ```tsx
 * <DropdownMenuStickySection $position="top">
 *   <DropdownMenuSearch placeholder="Search..." autoFocus />
 * </DropdownMenuStickySection>
 * {items}
 * <DropdownMenuStickySection $position="bottom">
 *   <Button onClick={handleApply}>Apply</Button>
 * </DropdownMenuStickySection>
 * ```
 */
export const DropdownMenuStickySection = styled(Box).attrs({
  paddingX: SECTION_PADDING_X,
})<{
  $position: 'top' | 'bottom';
}>`
  position: sticky;
  ${p => p.$position}: 0;
  left: 0;
  right: 0;
  ${p => (p.$position === 'bottom' ? `margin-top: ${p.theme.space[1]}px;` : '')}
  padding-top: ${p => p.theme.space[p.$position === 'top' ? 2 : 1]}px;
  padding-bottom: ${p => p.theme.space[p.$position === 'bottom' ? 2 : 1]}px;
  background-color: ${({ theme }) => theme.colors.levels.elevated};
  z-index: 10;

  /* Negate section padding if nested */
  ${StyledBodySection} & {
    margin-left: -${p => p.theme.space[SECTION_PADDING_X]}px;
    margin-right: -${p => p.theme.space[SECTION_PADDING_X]}px;
  }
`;

const StyledHeader = styled(Text).attrs({
  typography: 'body3',
  color: 'text.muted',
  paddingX: SECTION_PADDING_X,
  bold: true,
})`
  pointer-events: none;
  margin-top: ${({ theme }) => theme.space[2]}px;
  margin-bottom: ${({ theme }) => theme.space[1]}px;
  padding-top: ${({ theme }) => theme.space[2]}px;
  border-top: 1px solid ${({ theme }) => theme.colors.spotBackground[0]};

  /* Negate section padding if nested */
  ${StyledBodySection} & {
    margin-left: -${p => p.theme.space[SECTION_PADDING_X]}px;
    margin-right: -${p => p.theme.space[SECTION_PADDING_X]}px;
  }
  /* No bottom margin if before stickysection */
  &:has(+ ${DropdownMenuStickySection}) {
    margin-bottom: 0;
  }
  /* No top separator if first header */
  ${DropdownMenuStickySection} ~ ${StyledBodySection}:not(
    ${StyledBodySection} ~ ${StyledBodySection}
  ) > &:first-child,
  ${StyledBodySection} > ${DropdownMenuStickySection} ~ & {
    border-top: none;
    margin-top: ${({ theme }) => theme.space[2]}px;
    padding-top: 0;
  }
  ${StyledBodySection}:first-child > &:first-child {
    border-top: none;
    margin-top: ${({ theme }) => theme.space[1]}px;
    padding-top: 0;
  }
`;

export type DropdownMenuItemProps = HTMLAttributes<HTMLElement> & {
  label?: string;
  disabled?: boolean;
  as?: 'label' | 'button' | 'a';
  closeOnSelect?: boolean;
  $active?: boolean;
  // Anchor props, applicable when as="a"
  href?: string;
  target?: string;
  rel?: string;
};

/**
 * A single item within a `DropdownMenu`. Participates in keyboard
 * navigation and receives focus/hover styling automatically.
 *
 * Can render as a different element via the `as` prop (e.g., `"a"` for
 * links, `"label"` for checkbox wrappers). Pass `$active` to highlight
 * the item as selected independently of keyboard focus.
 *
 * Also used as a submenu trigger. Pass `ref` and `getReferenceProps()`
 * from a nested `DropdownMenu`'s `renderTrigger`.
 */
export const DropdownMenuItem = forwardRef<HTMLElement, DropdownMenuItemProps>(
  function DropdownMenuItem(
    {
      label,
      disabled,
      children,
      as = 'button',
      closeOnSelect = false,
      ...props
    },
    forwardedRef
  ) {
    const menu = useContext(DropdownMenuContext);
    const { ref: listItemRef, index } = useListItem({
      label: disabled ? null : label,
    });
    const isActive = index === menu.activeIndex;

    return (
      <StyledMenuItem
        as={as}
        ref={useMergeRefs([listItemRef, forwardedRef])}
        role="menuitem"
        type="button"
        tabIndex={isActive ? 0 : -1}
        aria-disabled={disabled}
        title={label}
        {...menu.getItemProps({
          ...props,
          onClick: e => {
            if (disabled) {
              e.preventDefault();
              return;
            }
            props.onClick?.(e);
            if (closeOnSelect) {
              menu.closeMenu();
            }
          },
        })}
      >
        {children ?? label}
      </StyledMenuItem>
    );
  }
);

/**
 * A menu item with a checkbox. Renders as a `<label>` wrapping a
 * checkbox input and text.
 *
 * @example
 * ```tsx
 * <DropdownMenuCheckableItem
 *   label="Enable notifications"
 *   checked={enabled}
 *   onChange={() => setEnabled(!enabled)}
 * />
 * ```
 */
export const DropdownMenuCheckableItem = ({
  label,
  disabled,
  checked,
  onChange,
  closeOnSelect,
  ...textProps
}: {
  label: string;
  disabled?: boolean;
  checked: boolean;
  onChange: React.ComponentProps<typeof CheckboxInput>['onChange'];
  closeOnSelect?: boolean;
} & React.ComponentProps<typeof Text>) => (
  <DropdownMenuItem
    as="label"
    label={label}
    title={label}
    disabled={disabled}
    role="menuitemcheckbox"
    aria-checked={checked}
    closeOnSelect={closeOnSelect}
  >
    <Flex flexDirection="row" alignItems="center" gap={2}>
      <Flex alignItems="center" justifyContent="center" aria-hidden="true">
        <CheckboxInput
          type="checkbox"
          checked={checked}
          onChange={onChange}
          disabled={disabled}
        />
      </Flex>
      <Text typography="body2" {...textProps}>
        {label}
      </Text>
    </Flex>
  </DropdownMenuItem>
);

const StyledMenuItem = styled(Box)<{ $active?: boolean }>`
  display: block;
  min-height: ${({ theme }) => theme.space[3]}px;
  width: -webkit-fill-available;
  margin: 0;
  padding: ${({ theme }) => theme.space[2]}px
    ${({ theme }) => theme.space[SECTION_PADDING_X]}px;
  user-select: none;
  cursor: pointer;
  text-decoration: none;
  text-overflow: ellipsis;
  overflow: hidden;
  color: ${({ theme }) => theme.colors.text.main};
  outline: none;
  background: ${({ theme, $active }) =>
    $active ? theme.colors.interactive.tonal.primary[0] : 'transparent'};
  border: none;
  transition:
    background-color 150ms ease,
    color 150ms ease;

  &:focus-visible,
  &:hover {
    background: ${({ theme }) => theme.colors.spotBackground[0]};
    color: ${({ theme }) => theme.colors.text.main};
  }

  &[aria-disabled='true'] {
    background: transparent;
    color: ${({ theme }) => theme.colors.text.muted};
    cursor: default;
  }

  /* Negate section padding if nested so clickable area spans full width */
  ${StyledBodySection} & {
    margin: 0 -${({ theme }) => theme.space[SECTION_PADDING_X]}px;
  }
`;

/**
 * A search input that reads and writes the current menu's filter state
 * from `DropdownMenuContext`. Wrap in `DropdownMenuStickySection`
 * to keep visible while menu content scrolls.
 *
 * @example
 * ```tsx
 * <DropdownMenuStickySection $position="top">
 *   <DropdownMenuSearch placeholder="Search items..." autoFocus />
 * </DropdownMenuStickySection>
 * ```
 */
export const DropdownMenuSearch = ({
  onChange,
  ...props
}: Omit<
  React.ComponentProps<typeof StyledSearchInput>,
  'value' | 'defaultValue'
>) => {
  const { search, setSearch } = useContext(DropdownMenuContext);

  return (
    <StyledSearchInput
      type="text"
      autoComplete="off"
      aria-label="Filter items"
      {...props}
      value={search}
      onChange={e => {
        setSearch(e.target.value);
        onChange?.(e);
      }}
    />
  );
};

const StyledSearchInput = styled.input`
  box-sizing: border-box;
  display: block;
  width: 100%;
  height: ${({ theme }) => theme.space[5]}px;
  padding: ${({ theme }) => theme.space[1]}px ${({ theme }) => theme.space[2]}px;
  border: 1px solid ${({ theme }) => theme.colors.buttons.border.active};
  border-radius: ${({ theme }) => theme.radii[2]}px;
  color: ${({ theme }) => theme.colors.text.main};
  background-color: transparent;
  outline: none;
  transition:
    border-color 150ms ease,
    background-color 150ms ease;

  &:focus-visible {
    border-color: ${({ theme }) => theme.colors.buttons.border.border};
  }
  &:focus-visible,
  &:hover {
    background-color: ${({ theme }) =>
      theme.colors.interactive.tonal.neutral[0]};
  }
  ${DropdownMenuStickySection} & {
    width: calc(100% + ${({ theme }) => theme.space[2]}px);
    margin: 0 -${({ theme }) => theme.space[1]}px;
  }
`;
