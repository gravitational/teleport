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

import React from 'react';
import { screen } from '@testing-library/react';

import { render, fireEvent } from 'design/utils/testing';

import SlideTabs from './SlideTabs';

describe('design/SlideTabs', () => {
  it('renders the supplied number of tabs(3)', () => {
    render(
      <SlideTabs
        tabs={['aws', 'automatically', 'manually']}
        onChange={() => {}}
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
      />
    );

    // eslint-disable-next-line testing-library/no-container, testing-library/no-node-access
    expect(container.querySelectorAll('input[name=pineapple]')).toHaveLength(3);
  });

  it('calls the onChange handler when the tab is changed', () => {
    const cb = jest.fn();
    render(
      <SlideTabs onChange={cb} tabs={['aws', 'automatically', 'manually']} />
    );
    fireEvent.click(screen.getByText('manually'));

    // The reason there are two calls to the callback is because when the
    // component is initially rendered it selects the first tab which is in
    // index 0 and calls the callback as such.
    expect(cb).toHaveBeenNthCalledWith(1, 0);
    expect(cb).toHaveBeenNthCalledWith(2, 2);
  });

  it('supports a square xlarge appearance (default)', () => {
    const { container } = render(
      <SlideTabs
        tabs={['aws', 'automatically', 'manually']}
        onChange={() => {}}
      />
    );
    expect(container).toMatchSnapshot();
  });

  it('supports a round xlarge appearance', () => {
    const { container } = render(
      <SlideTabs
        appearance="round"
        tabs={['aws', 'automatically', 'manually']}
        onChange={() => {}}
      />
    );
    expect(container).toMatchSnapshot();
  });

  it('supports a square medium size', () => {
    const { container } = render(
      <SlideTabs
        size="medium"
        tabs={['aws', 'automatically', 'manually']}
        onChange={() => {}}
      />
    );
    expect(container).toMatchSnapshot();
  });

  it('supports a round medium size', () => {
    const { container } = render(
      <SlideTabs
        size="medium"
        appearance="round"
        tabs={['aws', 'automatically', 'manually']}
        onChange={() => {}}
      />
    );
    expect(container).toMatchSnapshot();
  });

  it('supports passing in a selected index', () => {
    const { container } = render(
      <SlideTabs
        initialSelected={1}
        tabs={['aws', 'automatically', 'manually']}
        onChange={() => {}}
      />
    );
    expect(container).toMatchSnapshot();
  });
});
