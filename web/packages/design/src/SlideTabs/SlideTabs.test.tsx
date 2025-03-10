/**
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

import { screen } from '@testing-library/react';
import { useState } from 'react';

import * as Icon from 'design/Icon';
import { render, userEvent } from 'design/utils/testing';

import { SlideTabs, SlideTabsProps } from './SlideTabs';

describe('design/SlideTabs', () => {
  it('renders the supplied number of tabs(3)', () => {
    render(
      <SlideTabs
        tabs={[
          { key: 'aws', title: 'aws' },
          { key: 'automatically', title: 'automatically' },
          { key: 'manually', title: 'manually' },
        ]}
        onChange={() => {}}
        activeIndex={0}
      />
    );

    expect(screen.getAllByRole('tab')).toHaveLength(3);

    expect(getTab('aws')).toBeInTheDocument();
    expect(getTab('automatically')).toBeInTheDocument();
    expect(getTab('manually')).toBeInTheDocument();
  });

  it('renders the supplied number of tabs(5)', () => {
    render(
      <SlideTabs
        tabs={[
          { key: 'aws', title: 'aws' },
          { key: 'automatically', title: 'automatically' },
          { key: 'manually', title: 'manually' },
          { key: 'apple', title: 'apple' },
          { key: 'purple', title: 'purple' },
        ]}
        onChange={() => {}}
        activeIndex={0}
      />
    );

    expect(screen.getAllByRole('tab')).toHaveLength(5);

    expect(getTab('aws')).toBeInTheDocument();
    expect(getTab('automatically')).toBeInTheDocument();
    expect(getTab('manually')).toBeInTheDocument();
    expect(getTab('apple')).toBeInTheDocument();
    expect(getTab('purple')).toBeInTheDocument();
  });

  test('onChange highlights the tab clicked', async () => {
    render(<Component />);

    // First tab is selected by default.
    expect(getTab('first')).toHaveClass('selected');

    // Select the second tab.
    await userEvent.click(screen.getByText('second'));
    expect(getTab('second')).toHaveClass('selected');

    expect(getTab('first')).not.toHaveClass('selected');
  });

  test('keyboard navigation and accessibility', async () => {
    const user = userEvent.setup();
    render(
      <Component
        tabs={[
          { key: 'id1', title: 'first', controls: 'tabpanel-1' },
          {
            key: 'id2',
            icon: Icon.Check,
            ariaLabel: 'second',
            controls: 'tabpanel-2',
          },
        ]}
      />
    );
    expect(getTab('first')).not.toHaveFocus();
    expect(getTab('second')).not.toHaveFocus();

    getTab('first').focus();
    expect(getTab('first')).toHaveAttribute('aria-selected', 'true');
    expect(getTab('first')).toHaveAttribute('aria-controls', 'tabpanel-1');
    expect(getTab('second')).toHaveAttribute('aria-selected', 'false');
    expect(getTab('second')).toHaveAttribute('aria-controls', 'tabpanel-2');

    await user.keyboard('{Right}');
    expect(getTab('first')).toHaveAttribute('aria-selected', 'false');
    expect(getTab('second')).toHaveAttribute('aria-selected', 'true');
    expect(getTab('second')).toHaveFocus();

    // Should be a no-op.
    await user.keyboard('{Right}');
    expect(getTab('first')).toHaveAttribute('aria-selected', 'false');
    expect(getTab('second')).toHaveAttribute('aria-selected', 'true');
    expect(getTab('second')).toHaveFocus();

    await user.keyboard('{Left}');
    expect(getTab('first')).toHaveAttribute('aria-selected', 'true');
    expect(getTab('second')).toHaveAttribute('aria-selected', 'false');
    expect(getTab('first')).toHaveFocus();

    // Should be a no-op.
    await user.keyboard('{Left}');
    expect(getTab('first')).toHaveAttribute('aria-selected', 'true');
    expect(getTab('second')).toHaveAttribute('aria-selected', 'false');
    expect(getTab('first')).toHaveFocus();
  });
});

const Component = (props: Partial<SlideTabsProps>) => {
  const [activeIndex, setActiveIndex] = useState(0);

  return (
    <SlideTabs
      onChange={setActiveIndex}
      tabs={['first', 'second']}
      activeIndex={activeIndex}
      {...props}
    />
  );
};

const getTab = (name: string) => screen.getByRole('tab', { name });
