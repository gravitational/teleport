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

import { render } from 'design/utils/testing';

import ButtonIcon from './index';

describe('design/ButtonIcon', () => {
  it('renders a <button> and respects default "size" to 1', () => {
    const { container } = render(<ButtonIcon />);
    expect(container.firstChild.nodeName).toBe('BUTTON');
    expect(container.firstChild).toHaveStyle('font-size: 16px');
  });

  test('"size" 0 maps to font-size 12px', () => {
    const { container } = render(<ButtonIcon size={0} />);
    expect(container.firstChild).toHaveStyle('font-size: 12px');
  });

  test('"size" 1 maps to font-size 16px', () => {
    const { container } = render(<ButtonIcon size={1} />);
    expect(container.firstChild).toHaveStyle('font-size: 16px');
  });

  test('"size" 2 maps to font-size 24px', () => {
    const { container } = render(<ButtonIcon size={2} />);
    expect(container.firstChild).toHaveStyle('font-size: 24px');
  });
});
