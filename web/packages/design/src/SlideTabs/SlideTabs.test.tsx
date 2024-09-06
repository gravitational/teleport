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

import React, { useState } from 'react';
import { screen } from '@testing-library/react';

import { render, userEvent } from 'design/utils/testing';

import { SlideTabs } from './SlideTabs';

describe('design/SlideTabs', () => {
  it('renders the supplied number of tabs(3)', () => {
    render(
      <SlideTabs
        tabs={['aws', 'automatically', 'manually']}
        onChange={() => {}}
        activeIndex={0}
      />
    );

    expect(screen.getAllByRole('tab')).toHaveLength(3);

    expect(screen.getByLabelText('aws')).toBeInTheDocument();
    expect(screen.getByLabelText('automatically')).toBeInTheDocument();
    expect(screen.getByLabelText('manually')).toBeInTheDocument();
  });

  it('renders the supplied number of tabs(5)', () => {
    render(
      <SlideTabs
        tabs={['aws', 'automatically', 'manually', 'apple', 'purple']}
        onChange={() => {}}
        activeIndex={0}
      />
    );

    expect(screen.getAllByRole('tab')).toHaveLength(5);

    expect(screen.getByLabelText('aws')).toBeInTheDocument();
    expect(screen.getByLabelText('automatically')).toBeInTheDocument();
    expect(screen.getByLabelText('manually')).toBeInTheDocument();
    expect(screen.getByLabelText('apple')).toBeInTheDocument();
    expect(screen.getByLabelText('purple')).toBeInTheDocument();
  });

  it('respects a custom form name', () => {
    const { container } = render(
      <SlideTabs
        name="pineapple"
        tabs={['aws', 'automatically', 'manually']}
        onChange={() => {}}
        activeIndex={0}
      />
    );

    // eslint-disable-next-line testing-library/no-container, testing-library/no-node-access
    expect(container.querySelectorAll('input[name=pineapple]')).toHaveLength(3);
  });

  test('onChange highlights the tab clicked', async () => {
    render(<Component />);

    // First tab is selected by default.
    expect(screen.getByRole('tab', { name: 'first' })).toHaveClass('selected');

    // Select the second tab.
    await userEvent.click(screen.getByText('second'));
    expect(screen.getByRole('tab', { name: 'second' })).toHaveClass('selected');

    expect(screen.getByRole('tab', { name: 'first' })).not.toHaveClass(
      'selected'
    );
  });
});

const Component = () => {
  const [activeIndex, setActiveIndex] = useState(0);

  return (
    <SlideTabs
      onChange={setActiveIndex}
      tabs={['first', 'second']}
      activeIndex={activeIndex}
    />
  );
};
