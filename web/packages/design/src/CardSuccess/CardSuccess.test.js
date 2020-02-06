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
import CardSuccess from './index';
import { render } from 'design/utils/testing';

describe('design/CardSuccess', () => {
  it('renders checkmark icon', () => {
    const { container } = render(<CardSuccess />);

    expect(
      container
        .querySelector('span')
        .classList.contains('icon-checkmark-circle')
    ).toBe(true);
  });

  it('respects title prop and render text children', () => {
    const title = 'some title';
    const text = 'some text';
    const { container } = render(
      <CardSuccess title={title}>{text}</CardSuccess>
    );

    expect(container.firstChild.children[1].textContent).toBe(title);
    expect(container.firstChild.children[2].textContent).toBe(text);
  });
});
