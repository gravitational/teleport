/**
 * Copyright 2021 Gravitational, Inc.
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
import { render, screen, fireEvent } from 'design/utils/testing';
import { desktops } from '../fixtures';
import DesktopList from './DesktopList';

test('search filter works', () => {
  const searchValue = 'yetanother';
  const expectedToBeVisible = /bar: foo/i;
  const notExpectedToBeVisible = /d96e7dd6-26b6-56d5-8259-778f943f90f2/i;

  render(
    <DesktopList
      username="joe"
      desktops={desktops}
      clusterId="im-a-cluster"
      onLoginMenuOpen={() => null}
      onLoginSelect={() => null}
    />
  );

  fireEvent.change(screen.getByPlaceholderText(/SEARCH.../i), {
    target: { value: searchValue },
  });

  expect(screen.queryByText(expectedToBeVisible)).toBeInTheDocument();
  expect(screen.queryByText(notExpectedToBeVisible)).toBeNull();
});
