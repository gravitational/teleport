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

import { render, theme } from 'design/utils/testing';

import ButtonOutlined from './index';

describe('design/ButtonOutlined', () => {
  it('renders a <button> and respects default props', () => {
    const { container } = render(<ButtonOutlined />);
    expect(container.firstChild.nodeName).toBe('BUTTON');
    expect(container.firstChild).toHaveStyle('font-size: 12px');
    expect(container.firstChild).toHaveStyle({
      'border-color': theme.colors.text.primary,
    });
  });

  it('respects "size" small prop', () => {
    const { container } = render(<ButtonOutlined size="small" />);
    expect(container.firstChild).toHaveStyle('font-size: 10px');
  });

  it('respects "size" medium prop', () => {
    const { container } = render(<ButtonOutlined size="medium" />);
    expect(container.firstChild).toHaveStyle('font-size: 12px');
  });

  it('respects "size" large prop', () => {
    const { container } = render(<ButtonOutlined size="large" />);
    expect(container.firstChild).toHaveStyle('font-size: 12px');
  });

  it('respects "kind" primary prop', () => {
    const { container } = render(<ButtonOutlined kind="primary" />);
    expect(container.firstChild).toHaveStyle({
      'border-color': theme.colors.brand.main,
    });
  });
});
