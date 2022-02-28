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
import { kubes } from '../fixtures';
import KubeList from './KubeList';

test('search filter works', () => {
  const searchValue = 'tele.logicoma.dev-prod';
  const expectedToBeVisible = /env: prod/i;
  const notExpectedToBeVisible = /env: staging/i;

  render(
    <KubeList
      username="joe"
      authType="local"
      kubes={kubes}
      clusterId={'some-cluster-name'}
    />
  );

  fireEvent.change(screen.getByPlaceholderText(/SEARCH.../i), {
    target: { value: searchValue },
  });

  expect(screen.queryByText(expectedToBeVisible)).toBeInTheDocument();
  expect(screen.queryByText(notExpectedToBeVisible)).toBeNull();
});
