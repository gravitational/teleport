/**
 * Copyright 2023 Gravitational, Inc.
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
import { render, screen, userEvent } from 'design/utils/testing';

import { NavigationSwitcher } from './NavigationSwitcher';
import { NavigationCategory } from './categories';

test('not requiring attention', async () => {
  render(
    <NavigationSwitcher
      onChange={() => null}
      items={[
        { category: NavigationCategory.Management },
        { category: NavigationCategory.Resources },
      ]}
      value={NavigationCategory.Resources}
    />
  );

  expect(
    screen.queryByTestId('nav-switch-attention-dot')
  ).not.toBeInTheDocument();
  expect(screen.queryByTestId('dd-item-attention-dot')).not.toBeInTheDocument();

  // Test clicking
  await userEvent.click(screen.getByTestId('nav-switch-button'));
  expect(
    screen.queryByTestId('nav-switch-attention-dot')
  ).not.toBeInTheDocument();
  expect(screen.queryByTestId('dd-item-attention-dot')).not.toBeInTheDocument();
});

test('requires attention: not at nav category target (management)', async () => {
  render(
    <NavigationSwitcher
      onChange={() => null}
      items={[
        { category: NavigationCategory.Management, requiresAttention: true },
        { category: NavigationCategory.Resources },
      ]}
      value={NavigationCategory.Resources}
    />
  );

  expect(screen.getByTestId('nav-switch-attention-dot')).toBeInTheDocument();
  expect(screen.queryByTestId('dd-item-attention-dot')).not.toBeVisible();

  // Test clicking
  await userEvent.click(screen.getByTestId('nav-switch-button'));
  expect(screen.getByTestId('nav-switch-attention-dot')).toBeInTheDocument();
  expect(screen.getByTestId('dd-item-attention-dot')).toBeVisible();
});

test('requires attention: being at the nav category target (management) should NOT render attention dot', async () => {
  render(
    <NavigationSwitcher
      onChange={() => null}
      items={[
        { category: NavigationCategory.Management, requiresAttention: true },
        { category: NavigationCategory.Resources },
      ]}
      value={NavigationCategory.Management}
    />
  );

  expect(
    screen.queryByTestId('nav-switch-attention-dot')
  ).not.toBeInTheDocument();
  expect(screen.queryByTestId('dd-item-attention-dot')).not.toBeInTheDocument();

  // Test clicking
  await userEvent.click(screen.getByTestId('nav-switch-button'));
  expect(
    screen.queryByTestId('nav-switch-attention-dot')
  ).not.toBeInTheDocument();
  expect(screen.queryByTestId('dd-item-attention-dot')).not.toBeInTheDocument();
});
