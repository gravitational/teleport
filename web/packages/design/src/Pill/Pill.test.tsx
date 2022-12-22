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

import { Pill } from './Pill';

describe('design/Pill', () => {
  it('renders the label without dismissable', () => {
    render(<Pill label="arch: x86_64" />);

    expect(screen.getByText('arch: x86_64')).toBeInTheDocument();
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('render the label with dismissable', () => {
    render(<Pill label="arch: x86_64" onDismiss={() => {}} />);

    expect(screen.getByText('arch: x86_64')).toBeInTheDocument();
    expect(screen.getByRole('button')).toBeVisible();
  });

  it('dismissing pill calls onDismiss', () => {
    const cb = jest.fn();
    render(<Pill label="arch: x86_64" onDismiss={cb} />);

    fireEvent.click(screen.getByRole('button'));

    expect(cb).toHaveBeenCalledWith('arch: x86_64');
  });
});
