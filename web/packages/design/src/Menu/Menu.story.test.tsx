/**
 * Copyright 2020 Gravitational, Inc.
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

import { render } from 'design/utils/testing';

import { IconExample } from './Menu.story';

describe('design/Menu', () => {
  it('renders parent and its children', () => {
    render(<IconExample />);

    const parent = screen.getByTestId('Modal');
    const menu = screen.getByRole('menu');
    const item = screen.getAllByTestId('item');
    const icon = screen.getAllByTestId('icon');

    expect(parent).toBeInTheDocument();
    expect(menu).toBeInTheDocument();
    expect(item).toHaveLength(3);
    expect(icon).toHaveLength(3);
  });
});
