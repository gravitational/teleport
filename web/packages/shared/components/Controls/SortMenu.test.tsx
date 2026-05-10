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

import { UserEvent } from '@testing-library/user-event';
import { ComponentProps, PropsWithChildren } from 'react';

import darkTheme from 'design/theme/themes/darkTheme';
import { ConfiguredThemeProvider } from 'design/ThemeProvider';
import { render, screen, userEvent, within } from 'design/utils/testing';

import { SortMenu } from './SortMenu';

describe('SortMenu', () => {
  it('shows the selected item in the button', async () => {
    renderComponent();
    expect(screen.getByText('Name')).toBeInTheDocument();
  });

  describe('shows the selected item sort-specific in the button', () => {
    it('ascending', async () => {
      renderComponent({
        props: {
          items: [
            {
              key: 'test',
              label: 'Test',
              ascendingLabel: 'Test ASC',
            },
          ],
          selectedKey: 'test',
          selectedOrder: 'ASC',
        },
      });
      expect(screen.getByText('Test ASC')).toBeInTheDocument();
    });

    it('descending', async () => {
      renderComponent({
        props: {
          items: [
            {
              key: 'test',
              label: 'Test',
              descendingLabel: 'Test DESC',
            },
          ],
          selectedKey: 'test',
          selectedOrder: 'DESC',
        },
      });
      expect(screen.getByText('Test DESC')).toBeInTheDocument();
    });
  });

  it('shows the selected order in the button', async () => {
    renderComponent();
    expect(screen.getByTestId('sort-asc-icon')).toBeInTheDocument();
  });

  it('shows the options menu', async () => {
    const { user } = renderComponent();
    await openMenu(user);
    expect(screen.getByRole('menu')).toBeInTheDocument();
  });

  it('shows the options menu items', async () => {
    const { user } = renderComponent();
    await openMenu(user);
    const menu = screen.getByRole('menu');
    expect(
      within(menu).getByRole('menuitem', { name: 'Name' })
    ).toBeInTheDocument();
    expect(
      within(menu).getByRole('menuitem', { name: 'Created' })
    ).toBeInTheDocument();
    expect(
      within(menu).getByRole('menuitem', { name: 'Relevance' })
    ).toBeInTheDocument();
  });

  it('shows the options menu order options', async () => {
    const { user } = renderComponent();
    await openMenu(user);
    const menu = screen.getByRole('menu');
    expect(
      within(menu).getByRole('menuitem', { name: 'Ascending' })
    ).toBeInTheDocument();
    expect(
      within(menu).getByRole('menuitem', { name: 'Descending' })
    ).toBeInTheDocument();
  });

  it('shows the options menu order options with custom labels', async () => {
    const { user } = renderComponent({
      props: {
        items: [
          {
            key: 'test',
            label: 'Test',
            ascendingOptionLabel: 'Test ASC',
            descendingOptionLabel: 'Test DESC',
          },
        ],
        selectedKey: 'test',
        selectedOrder: 'DESC',
      },
    });
    await openMenu(user, 'Test');
    const menu = screen.getByRole('menu');
    expect(
      within(menu).getByRole('menuitem', { name: 'Test ASC' })
    ).toBeInTheDocument();
    expect(
      within(menu).getByRole('menuitem', { name: 'Test DESC' })
    ).toBeInTheDocument();
  });

  it('shows disabled options menu order options', async () => {
    const { user, onChange } = renderComponent({
      props: {
        items: [
          {
            key: 'test',
            label: 'Test',
            disableSort: true,
          },
        ],
        selectedKey: 'test',
        selectedOrder: 'DESC',
      },
    });
    await openMenu(user, 'Test');
    const menu = screen.getByRole('menu');
    const asc = within(menu).getByRole('menuitem', { name: 'Ascending' });
    const desc = within(menu).getByRole('menuitem', { name: 'Descending' });
    expect(asc).toBeInTheDocument();
    expect(desc).toBeInTheDocument();
    await user.click(asc);
    await user.click(desc);
    expect(onChange).not.toHaveBeenCalled();
  });

  it('allows an item to be selected', async () => {
    const { user, onChange } = renderComponent();
    await openMenu(user);
    const menu = screen.getByRole('menu');
    const name = within(menu).getByRole('menuitem', { name: 'Created' });
    expect(name).toBeInTheDocument();
    await user.click(name);
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenLastCalledWith('created', 'ASC');
  });

  it('allows an order to be selected', async () => {
    const { user, onChange } = renderComponent();
    await openMenu(user);
    const menu = screen.getByRole('menu');
    const name = within(menu).getByRole('menuitem', { name: 'Descending' });
    expect(name).toBeInTheDocument();
    await user.click(name);
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenLastCalledWith('name', 'DESC');
  });

  it('applies a default sort order when an item is selected', async () => {
    const { user, onChange } = renderComponent({
      props: {
        items: [
          {
            key: 'test-1',
            label: 'Test 1',
          },
          {
            key: 'test-2',
            label: 'Test 2',
            defaultOrder: 'DESC',
          },
        ],
        selectedKey: 'test-1',
        selectedOrder: 'ASC',
      },
    });
    await openMenu(user, 'Test 1');
    const menu = screen.getByRole('menu');
    const name = within(menu).getByRole('menuitem', { name: 'Test 2' });
    expect(name).toBeInTheDocument();
    await user.click(name);
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenLastCalledWith('test-2', 'DESC');
  });
});

async function openMenu(user: UserEvent, label = 'Name') {
  await user.click(screen.getByText(label));
}

const renderComponent = (options?: {
  props: Partial<ComponentProps<typeof SortMenu>>;
}) => {
  const { props } = options ?? {};
  const {
    items = [
      {
        key: 'name',
        label: 'Name',
      },
      {
        key: 'created',
        label: 'Created',
      },
      {
        key: 'relevance',
        label: 'Relevance',
      },
    ],
    onChange = jest.fn(),
    selectedKey = 'name',
    selectedOrder = 'ASC',
  } = props ?? {};

  const user = userEvent.setup();
  return {
    ...render(
      <SortMenu
        items={items}
        onChange={onChange}
        selectedKey={selectedKey}
        selectedOrder={selectedOrder}
      />,
      {
        wrapper: makeWrapper(),
      }
    ),
    onChange,
    user,
  };
};

function makeWrapper() {
  return (props: PropsWithChildren) => {
    return (
      <ConfiguredThemeProvider theme={darkTheme}>
        {props.children}
      </ConfiguredThemeProvider>
    );
  };
}
