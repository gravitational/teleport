/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import userEvent from '@testing-library/user-event';
import { PropsWithChildren, ReactNode } from 'react';
import {
  createMemoryRouter,
  RouterProvider,
  type InitialEntry,
} from 'react-router';

import { render } from 'design/utils/testing';

type RouterWrapper = React.FC<PropsWithChildren>;

type RenderWithMemoryRouterOptions = {
  path?: string;
  initialEntries?: InitialEntry[];
  initialIndex?: number;
  wrapper?: RouterWrapper;
};

export function renderWithMemoryRouter(
  ui: ReactNode,
  options?: RenderWithMemoryRouterOptions
) {
  const {
    path = '*',
    initialEntries = ['/'],
    initialIndex,
    wrapper: Wrapper,
  } = options ?? {};

  const element = Wrapper ? <Wrapper>{ui}</Wrapper> : ui;

  const router = createMemoryRouter(
    [
      {
        path,
        element,
      },
    ],
    {
      initialEntries,
      initialIndex,
    }
  );

  return {
    user: userEvent.setup(),
    router,
    ...render(<RouterProvider router={router} />),
  };
}
