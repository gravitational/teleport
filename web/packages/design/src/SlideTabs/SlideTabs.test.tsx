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

import { render, fireEvent } from 'design/utils/testing';

import SlideTabs from './SlideTabs';

describe('design/SlideTabs', () => {
  it('renders the supplied number of tabs(3)', () => {
    const { container } = render(
      <SlideTabs
        tabs={['aws', 'automatically', 'manually']}
        onChange={() => {}}
      />
    );
    expect(container.getElementsByTagName('label')).toHaveLength(3);
  });

  it('renders the supplied number of tabs(5)', () => {
    const { container } = render(
      <SlideTabs
        tabs={['aws', 'automatically', 'manually', 'apple', 'purple']}
        onChange={() => {}}
      />
    );
    expect(container.getElementsByTagName('label')).toHaveLength(5);
  });

  it('respects a custom form name', () => {
    const { container } = render(
      <SlideTabs
        name="pineapple"
        tabs={['aws', 'automatically', 'manually']}
        onChange={() => {}}
      />
    );
    expect(container.querySelectorAll('input[name=pineapple]')).toHaveLength(3);
  });

  it('calls the onChange handler when the tab is changed', () => {
    const cb = jest.fn();
    const { container } = render(
      <SlideTabs onChange={cb} tabs={['aws', 'automatically', 'manually']} />
    );
    fireEvent.click(container.querySelector('label[for=slide-tab-manually]'));
    expect(cb.mock.calls).toHaveLength(2);
    // The reason there are two calls to the callback is because when the
    // component is initially rendered it selects the first tab which is in
    // index 0 and calls the callback as such.
    expect(cb.mock.calls).toEqual([[0], [2]]);
  });

  it('supports a square xlarge appearance (default)', () => {
    const { container } = render(
      <SlideTabs
        tabs={['aws', 'automatically', 'manually']}
        onChange={() => {}}
      />
    );
    expect(container.firstChild).toMatchSnapshot();
  });

  it('supports a round xlarge appearance', () => {
    const { container } = render(
      <SlideTabs
        appearance="round"
        tabs={['aws', 'automatically', 'manually']}
        onChange={() => {}}
      />
    );
    expect(container.firstChild).toMatchSnapshot();
  });

  it('supports a square medium size', () => {
    const { container } = render(
      <SlideTabs
        size="medium"
        tabs={['aws', 'automatically', 'manually']}
        onChange={() => {}}
      />
    );
    expect(container.firstChild).toMatchSnapshot();
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
    expect(container.firstChild).toMatchSnapshot();
  });

  it('supports passing in a selected index', () => {
    const { container } = render(
      <SlideTabs
        initialSelected={1}
        tabs={['aws', 'automatically', 'manually']}
        onChange={() => {}}
      />
    );
    expect(container.firstChild).toMatchSnapshot();
  });
});
