/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { render, screen } from 'design/utils/testing';
import { makeSuccessAttempt } from 'shared/hooks/useAsync';

import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';

import * as pickers from './pickers/pickers';
import * as useSearchAttempts from './pickers/useSearchAttempts';
import * as SearchContext from './SearchContext';

import { SearchBarConnected } from './SearchBar';

it('does not display empty results copy after selecting two filters', () => {
  const appContext = new MockAppContext();
  appContext.workspacesService.setState(draft => {
    draft.rootClusterUri = '/clusters/foo';
  });

  const mockAttempts = [makeSuccessAttempt([])];
  jest
    .spyOn(useSearchAttempts, 'useSearchAttempts')
    .mockImplementation(() => mockAttempts);
  jest.spyOn(SearchContext, 'useSearchContext').mockImplementation(() => ({
    filters: [
      { filter: 'cluster', clusterUri: '/clusters/foo' },
      { filter: 'resource-type', resourceType: 'servers' },
    ],
    inputValue: '',
    setFilter: () => {},
    removeFilter: () => {},
    opened: true,
    open: () => {},
    close: () => {},
    closeAndResetInput: () => {},
    resetInput: () => {},
    changeActivePicker: () => {},
    onInputValueChange: () => {},
    activePicker: pickers.actionPicker,
    inputRef: undefined,
  }));

  render(
    <MockAppContextProvider appContext={appContext}>
      <SearchBarConnected />
    </MockAppContextProvider>
  );

  const results = screen.getByRole('menu');
  expect(results).not.toHaveTextContent('No matching results found');
});

it('does display empty results copy after providing search query for which there is no results', () => {
  const appContext = new MockAppContext();
  appContext.workspacesService.setState(draft => {
    draft.rootClusterUri = '/clusters/foo';
  });

  const mockAttempts = [makeSuccessAttempt([])];
  jest
    .spyOn(useSearchAttempts, 'useSearchAttempts')
    .mockImplementation(() => mockAttempts);
  jest.spyOn(SearchContext, 'useSearchContext').mockImplementation(() => ({
    inputValue: 'foo',
    filters: [],
    setFilter: () => {},
    removeFilter: () => {},
    opened: true,
    open: () => {},
    close: () => {},
    closeAndResetInput: () => {},
    resetInput: () => {},
    changeActivePicker: () => {},
    onInputValueChange: () => {},
    activePicker: pickers.actionPicker,
    inputRef: undefined,
  }));

  render(
    <MockAppContextProvider appContext={appContext}>
      <SearchBarConnected />
    </MockAppContextProvider>
  );

  const results = screen.getByRole('menu');
  expect(results).toHaveTextContent('No matching results found');
});
