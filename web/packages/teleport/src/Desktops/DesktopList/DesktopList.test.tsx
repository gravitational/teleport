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
import { render, screen, fireEvent } from 'design/utils/testing';

import { desktops } from '../fixtures';

import { props } from '../Desktops.story';

import DesktopList from './DesktopList';

test('search generates correct url params', () => {
  const replaceHistory = jest.fn();

  render(
    <DesktopList
      {...props}
      username="joe"
      desktops={desktops}
      clusterId="im-a-cluster"
      pathname="test.com/cluster/one/desktops"
      onLoginMenuOpen={() => null}
      onLoginSelect={() => null}
      replaceHistory={replaceHistory}
    />
  );

  fireEvent.change(screen.getByPlaceholderText(/SEARCH.../i), {
    target: { value: 'test' },
  });

  fireEvent.submit(screen.getByPlaceholderText(/SEARCH.../i));

  expect(replaceHistory).toHaveBeenCalledWith(
    'test.com/cluster/one/desktops?search=test&sort=name%3Aasc'
  );
});
