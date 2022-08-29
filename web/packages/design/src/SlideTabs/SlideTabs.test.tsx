/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
