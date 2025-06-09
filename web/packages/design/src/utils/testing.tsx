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

import {
  act,
  fireEvent,
  getByTestId,
  prettyDOM,
  screen,
  render as testingRender,
  waitFor,
  waitForElementToBeRemoved,
  within,
} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { PropsWithChildren, ReactNode } from 'react';
import { MemoryRouter as Router } from 'react-router-dom';

import { darkTheme } from 'design/theme';
import { ConfiguredThemeProvider } from 'design/ThemeProvider';

import '@testing-library/jest-dom';
import 'jest-styled-components';

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

export const testQueryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: false,
    },
  },
});

export function Providers({ children }: { children: ReactNode }) {
  return (
    <QueryClientProvider client={testQueryClient}>
      <ConfiguredThemeProvider theme={darkTheme}>
        {children}
      </ConfiguredThemeProvider>
    </QueryClientProvider>
  );
}

function render(
  ui: ReactNode,
  options?: RenderOptions
): ReturnType<typeof testingRender> {
  return testingRender(ui, { wrapper: Providers, ...options });
}

/*
 Returns a Promise resolving on the next macrotask, allowing any pending state
 updates / timeouts to finish.
 */
function tick() {
  return new Promise<void>(res =>
    jest.requireActual('timers').setImmediate(res)
  );
}

screen.debug = () => {
  window.console.log(prettyDOM());
};

type RenderOptions = {
  wrapper?: React.FC<PropsWithChildren>;
  container?: HTMLElement;
};

export {
  act,
  screen,
  fireEvent,
  darkTheme as theme,
  tick,
  render,
  prettyDOM,
  waitFor,
  getByTestId,
  Router,
  userEvent,
  waitForElementToBeRemoved,
  within,
};
