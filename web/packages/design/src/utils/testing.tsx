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
import {
  render as testingRender,
  act,
  fireEvent,
  waitFor,
  screen,
  prettyDOM,
  getByTestId,
  waitForElementToBeRemoved,
} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter as Router } from 'react-router-dom';

import ThemeProvider from 'design/ThemeProvider';
import { darkTheme } from 'design/theme';
import '@testing-library/jest-dom';
import 'jest-styled-components';

function Providers({ children }: { children: React.ReactElement }) {
  return <ThemeProvider theme={darkTheme}>{children}</ThemeProvider>;
}

function render(ui: React.ReactElement<any>, options?: RenderOptions) {
  return testingRender(ui, { wrapper: Providers, ...options });
}

screen.debug = () => {
  window.console.log(prettyDOM());
};

type RenderOptions = {
  wrapper: React.FC;
  container: HTMLElement;
};

export {
  act,
  screen,
  fireEvent,
  darkTheme as theme,
  render,
  prettyDOM,
  waitFor,
  getByTestId,
  Router,
  userEvent,
  waitForElementToBeRemoved,
};
