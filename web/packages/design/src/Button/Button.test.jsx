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

import Button, { ButtonPrimary, ButtonSecondary, ButtonWarning } from './index';

describe('design/Button', () => {
  it('renders a <button> and respects default "kind" prop == primary', () => {
    const { container } = render(<Button />);
    expect(container.firstChild.nodeName).toBe('BUTTON');
    expect(container.firstChild).toHaveStyle({
      background: theme.colors.brand.main,
    });
  });

  test('"kind" primary renders bg == theme.colors.brand.main', () => {
    const { container } = render(<ButtonPrimary />);
    expect(container.firstChild).toHaveStyle({
      background: theme.colors.brand.main,
    });
  });

  test('"kind" secondary renders bg == theme.colors.levels.surface', () => {
    const { container } = render(<ButtonSecondary />);
    expect(container.firstChild).toHaveStyle({
      background: theme.colors.levels.surface,
    });
  });

  test('"kind" warning renders bg == theme.colors.error.dark', () => {
    const { container } = render(<ButtonWarning />);
    expect(container.firstChild).toHaveStyle({
      background: theme.colors.error.dark,
    });
  });

  test('"size" small renders min-height: 24px', () => {
    const { container } = render(<Button size="small" />);
    expect(container.firstChild).toHaveStyle({ 'min-height': '24px' });
  });

  test('"size" medium renders min-height: 32px', () => {
    const { container } = render(<Button size="medium" />);
    expect(container.firstChild).toHaveStyle('min-height: 32px');
  });

  test('"size" large renders min-height: 40px', () => {
    const { container } = render(<Button size="large" />);
    expect(container.firstChild).toHaveStyle('min-height: 40px');
  });

  test('"block" prop renders width 100%', () => {
    const { container } = render(<Button block />);
    expect(container.firstChild).toHaveStyle('width: 100%');
  });
});
