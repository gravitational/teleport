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

import { Pill } from './Pill';

describe('design/Pill', () => {
  it('renders the label without dismissable', () => {
    const { container } = render(<Pill label="arch: x86_64" />);
    expect(container).toHaveTextContent('arch: x86_64');
    expect(container.getElementsByTagName('button')).toMatchSnapshot();
  });

  it('render the label with dismissable', () => {
    const { container } = render(
      <Pill label="arch: x86_64" onDismiss={() => {}} />
    );
    expect(container).toHaveTextContent('arch: x86_64');
    expect(container.getElementsByTagName('button')).toMatchSnapshot();
  });

  it('dismissing pill calls onDismiss', () => {
    const cb = jest.fn();
    const { container } = render(<Pill label="arch: x86_64" onDismiss={cb} />);
    fireEvent.click(container.querySelector('button'));
    expect(cb.mock.calls).toHaveLength(1);
    expect(cb.mock.calls).toEqual([['arch: x86_64']]);
  });
});
