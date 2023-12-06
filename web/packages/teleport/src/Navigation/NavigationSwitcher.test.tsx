/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
