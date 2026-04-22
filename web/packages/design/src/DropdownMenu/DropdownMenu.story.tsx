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

import { Placement } from '@floating-ui/react';
import type { Meta, StoryFn, StoryObj } from '@storybook/react-vite';
import { useContext } from 'react';
import { action } from 'storybook/actions';
import { useArgs } from 'storybook/preview-api';
import styled from 'styled-components';

import { ButtonBorder, ButtonPrimary, Flex, Text } from 'design';
import { ChevronDown, ChevronRight } from 'design/Icon';

import {
  DropdownMenu,
  DropdownMenuSection,
  DropdownMenuCheckableItem,
  DropdownMenuContext,
  DropdownMenuItem,
  DropdownMenuSearch,
  DropdownMenuStickySection,
  DropdownMenuPanel,
} from '.';

const allItems = [
  'Alpha',
  'Bravo',
  'Charlie',
  'Delta',
  'Echo',
  'Foxtrot',
  'Golf',
  'Hotel',
  'India',
  'Juliet',
  'verylongresourceprincipalnameforoverflow',
];

const disabledItems = new Set(['Delta', 'Hotel']);

type Args = {
  showSearch: boolean;
  showSections: boolean;
  useCheckboxes: boolean;
  showStickyFooter: boolean;
  showNestedMenu: boolean;
  showDisabledItems: boolean;
  hoverTrigger: boolean;
  placement: Placement;
  selected: string[];
};

export default {
  title: 'Design/DropdownMenu',
  argTypes: {
    showSearch: {
      control: 'boolean',
      description: 'Show a search input in a sticky top section',
    },
    showSections: {
      control: 'boolean',
      description: 'Split items into two sections with headers',
    },
    useCheckboxes: {
      control: 'boolean',
      description: 'Render items as checkboxes with selection state',
    },
    showStickyFooter: {
      control: 'boolean',
      description: 'Show a sticky footer with an action button',
    },
    showNestedMenu: {
      control: 'boolean',
      description: 'Include a nested submenu trigger',
    },
    showDisabledItems: {
      control: 'boolean',
      description: 'Mark Delta and Hotel as disabled',
    },
    hoverTrigger: {
      control: 'boolean',
      description: 'Open menu on hover in addition to click',
    },
    placement: {
      control: 'select',
      options: [
        'bottom-end',
        'bottom-start',
        'top-end',
        'top-start',
        'right-start',
        'left-start',
      ],
    },
    selected: { control: false },
  },
  args: {
    showSearch: false,
    showSections: false,
    useCheckboxes: false,
    showStickyFooter: false,
    showNestedMenu: false,
    showDisabledItems: false,
    hoverTrigger: false,
    placement: 'bottom-end',
    selected: [],
  },
  render: (args => {
    const [{ selected }, updateArgs] = useArgs<Args>();

    const toggleItem = (item: string) => {
      const next = selected.includes(item)
        ? selected.filter(s => s !== item)
        : [...selected, item];
      updateArgs({ selected: next });
      action('onToggle')(item, next);
    };

    return (
      <Flex alignItems="center" justifyContent="center" minHeight="300px">
        <DropdownMenu
          placement={args.placement}
          hoverTrigger={args.hoverTrigger}
          onOpenChange={action('onOpenChange')}
          panelComponent={CustomPanelComponent}
          renderTrigger={({ ref, getReferenceProps }) => (
            <ButtonBorder
              ref={ref}
              textTransform="none"
              size="small"
              css={`
                transition:
                  background 150ms ease,
                  border-color 150ms ease;
              `}
              {...getReferenceProps()}
            >
              Open Menu
              <ChevronDown ml={1} size="small" />
            </ButtonBorder>
          )}
        >
          <MenuContent {...args} selected={selected} onToggle={toggleItem} />
        </DropdownMenu>
      </Flex>
    );
  }) satisfies StoryFn<Args>,
} satisfies Meta<Args>;

const CustomPanelComponent = styled(DropdownMenuPanel)`
  max-width: 220px;
`;

type Story = StoryObj<Args>;

export const Default: Story = {};

export const Checkboxes: Story = {
  args: {
    showSearch: true,
    useCheckboxes: true,
    showStickyFooter: true,
  },
};

export const Nested: Story = {
  args: {
    showNestedMenu: true,
    showSections: true,
  },
};

const MenuContent = ({
  showSearch,
  showSections,
  useCheckboxes,
  showStickyFooter,
  showNestedMenu,
  showDisabledItems,
  selected,
  onToggle,
}: Omit<Args, 'hoverTrigger' | 'placement'> & {
  onToggle: (item: string) => void;
}) => {
  const { search } = useContext(DropdownMenuContext);
  const q = search.trim().toLowerCase();

  const items = q
    ? allItems.filter(item => item.toLowerCase().includes(q))
    : allItems;

  const midpoint = Math.ceil(items.length / 2);

  const renderItem = (item: string) => {
    const disabled = showDisabledItems && disabledItems.has(item);
    if (useCheckboxes) {
      return (
        <DropdownMenuCheckableItem
          key={item}
          label={item}
          disabled={disabled}
          checked={selected.includes(item)}
          onChange={() => onToggle(item)}
        />
      );
    }
    return (
      <DropdownMenuItem
        key={item}
        label={item}
        disabled={disabled}
        onClick={() => action('onItemClick')(item)}
      >
        {item}
      </DropdownMenuItem>
    );
  };

  return (
    <>
      {showSearch && (
        <DropdownMenuStickySection $position="top">
          <DropdownMenuSearch placeholder="Search items..." autoFocus />
        </DropdownMenuStickySection>
      )}

      {items.length > 0 &&
        (showSections ? (
          <>
            <DropdownMenuSection header="Group A">
              {items.slice(0, midpoint).map(renderItem)}
            </DropdownMenuSection>
            <DropdownMenuSection header="Group B">
              {items.slice(midpoint).map(renderItem)}
            </DropdownMenuSection>
          </>
        ) : (
          <DropdownMenuSection>{items.map(renderItem)}</DropdownMenuSection>
        ))}

      {showNestedMenu && (
        <DropdownMenuSection header="Nested Menu">
          <DropdownMenu
            placement="right-start"
            renderTrigger={({ ref, getReferenceProps }) => (
              <DropdownMenuItem ref={ref} {...getReferenceProps()}>
                <Flex
                  flex="1"
                  justifyContent="space-between"
                  alignItems="center"
                >
                  <Text>More options</Text>
                  <ChevronRight size="small" color="text.muted" />
                </Flex>
              </DropdownMenuItem>
            )}
          >
            <DropdownMenuSection>
              {['Sub-item A', 'Sub-item B', 'Sub-item C'].map(item => (
                <DropdownMenuItem
                  key={item}
                  label={item}
                  onClick={() => action('onItemClick')(item)}
                >
                  {item}
                </DropdownMenuItem>
              ))}
            </DropdownMenuSection>
          </DropdownMenu>
        </DropdownMenuSection>
      )}

      {items.length === 0 && <DropdownMenuSection header="No results" />}

      {showStickyFooter && (
        <DropdownMenuStickySection $position="bottom">
          <ButtonPrimary
            size="small"
            block
            onClick={() => action('onApply')(selected)}
          >
            Apply ({selected.length} selected)
          </ButtonPrimary>
        </DropdownMenuStickySection>
      )}
    </>
  );
};
