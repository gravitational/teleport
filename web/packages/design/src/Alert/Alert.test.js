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

import Alert, { Danger, Info, Warning, Success } from './index';

describe('design/Alert', () => {
  it('respects default "kind" prop == danger', () => {
    const { container } = render(<Alert />);
    expect(container.firstChild).toHaveStyle({
      background: theme.colors.danger,
    });
  });

  test('"kind" danger renders bg == theme.colors.danger', () => {
    const { container } = render(<Danger />);
    expect(container.firstChild).toHaveStyle({
      background: theme.colors.danger,
    });
  });

  test('"kind" warning renders bg == theme.colors.warning.main', () => {
    const { container } = render(<Warning />);
    expect(container.firstChild).toHaveStyle({
      background: theme.colors.warning.main,
    });
  });

  test('"kind" info renders bg == theme.colors.info', () => {
    const { container } = render(<Info />);
    expect(container.firstChild).toHaveStyle({
      background: theme.colors.info,
    });
  });

  test('"kind" success renders bg == theme.colors.success', () => {
    const { container } = render(<Success />);
    expect(container.firstChild).toHaveStyle({
      background: theme.colors.success,
    });
  });
});
