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
import userEvent from '@testing-library/user-event';
import { render, screen, waitFor, act } from 'design/utils/testing';
import { makeSuccessAttempt } from 'shared/hooks/useAsync';

import Logger, { NullService } from 'teleterm/logger';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { ResourceSearchError } from 'teleterm/ui/services/resources';
import ModalsHost from 'teleterm/ui/ModalsHost';
import {
  makeRootCluster,
  makeRetryableError,
} from 'teleterm/services/tshd/testHelpers';
import { ClusterUri } from 'teleterm/ui/uri';
import { VnetContextProvider } from 'teleterm/ui/Vnet';
import { ConnectionsContextProvider } from 'teleterm/ui/TopBar/Connections/connectionsContext';

import { SearchAction } from './actions';

import * as pickers from './pickers/pickers';
import * as useActionAttempts from './pickers/useActionAttempts';
import * as useSearch from './useSearch';
import * as SearchContext from './SearchContext';

import { SearchBarConnected } from './SearchBar';

beforeAll(() => {
  Logger.init(new NullService());
});

beforeEach(() => {
  jest.restoreAllMocks();
});

const displayResultsAction: SearchAction = {
  type: 'simple-action',
  searchResult: {
    kind: 'display-results',
    value: '',
    resourceKinds: [],
    clusterUri: '/clusters/foo',
    documentUri: undefined,
  },
  perform() {},
};

it('does not display empty results copy after selecting two filters', () => {
  const appContext = setUpContext('/clusters/foo');

  const mockActionAttempts = {
    displayResultsAction,
    filterActions: [],
    resourceActionsAttempt: makeSuccessAttempt([]),
    resourceSearchAttempt: makeSuccessAttempt({
      results: [],
      errors: [],
      search: '',
    }),
  };
  jest
    .spyOn(useActionAttempts, 'useActionAttempts')
    .mockImplementation(() => mockActionAttempts);
  jest.spyOn(SearchContext, 'useSearchContext').mockImplementation(() => ({
    ...getMockedSearchContext(),
    filters: [
      { filter: 'cluster', clusterUri: '/clusters/foo' },
      { filter: 'resource-type', resourceType: 'node' },
    ],
    inputValue: '',
  }));

  render(
    <MockAppContextProvider appContext={appContext}>
      <ConnectionsContextProvider>
        <VnetContextProvider>
          <SearchBarConnected />
        </VnetContextProvider>
      </ConnectionsContextProvider>
    </MockAppContextProvider>
  );

  const results = screen.getByRole('menu');
  expect(results).not.toHaveTextContent('No matching results found');
});

it('displays empty results copy after providing search query for which there is no results', () => {
  const appContext = setUpContext('/clusters/foo');

  const mockActionAttempts = {
    displayResultsAction,
    filterActions: [],
    resourceActionsAttempt: makeSuccessAttempt([]),
    resourceSearchAttempt: makeSuccessAttempt({
      results: [],
      errors: [],
      search: '',
    }),
  };
  jest
    .spyOn(useActionAttempts, 'useActionAttempts')
    .mockImplementation(() => mockActionAttempts);
  jest
    .spyOn(SearchContext, 'useSearchContext')
    .mockImplementation(getMockedSearchContext);

  render(
    <MockAppContextProvider appContext={appContext}>
      <ConnectionsContextProvider>
        <VnetContextProvider>
          <SearchBarConnected />
        </VnetContextProvider>
      </ConnectionsContextProvider>
    </MockAppContextProvider>
  );

  const results = screen.getByRole('menu');
  expect(results).toHaveTextContent('No matching results found.');
});

it('includes offline cluster names in the empty results copy', () => {
  const cluster = makeRootCluster({ connected: false });
  const appContext = setUpContext(cluster.uri);
  appContext.clustersService.setState(draftState => {
    draftState.clusters.set(cluster.uri, cluster);
  });

  const mockActionAttempts = {
    displayResultsAction,
    filterActions: [],
    resourceActionsAttempt: makeSuccessAttempt([]),
    resourceSearchAttempt: makeSuccessAttempt({
      results: [],
      errors: [],
      search: '',
    }),
  };
  jest
    .spyOn(useActionAttempts, 'useActionAttempts')
    .mockImplementation(() => mockActionAttempts);
  jest
    .spyOn(SearchContext, 'useSearchContext')
    .mockImplementation(getMockedSearchContext);

  render(
    <MockAppContextProvider appContext={appContext}>
      <ConnectionsContextProvider>
        <VnetContextProvider>
          <SearchBarConnected />
        </VnetContextProvider>
      </ConnectionsContextProvider>
    </MockAppContextProvider>
  );

  const results = screen.getByRole('menu');
  expect(results).toHaveTextContent('No matching results found.');
  expect(results).toHaveTextContent(
    `The cluster ${cluster.name} was excluded from the search because you are not logged in to it.`
  );
});

it('notifies about resource search errors and allows to display details', () => {
  const appContext = setUpContext('/clusters/foo');

  const resourceSearchError = new ResourceSearchError(
    '/clusters/foo',
    new Error('whoops')
  );

  const mockActionAttempts = {
    displayResultsAction,
    filterActions: [],
    resourceActionsAttempt: makeSuccessAttempt([]),
    resourceSearchAttempt: makeSuccessAttempt({
      results: [],
      errors: [resourceSearchError],
      search: '',
    }),
  };
  jest
    .spyOn(useActionAttempts, 'useActionAttempts')
    .mockImplementation(() => mockActionAttempts);
  const mockedSearchContext = {
    ...getMockedSearchContext(),
    inputValue: 'foo',
  };
  jest
    .spyOn(SearchContext, 'useSearchContext')
    .mockImplementation(() => mockedSearchContext);
  jest.spyOn(appContext.modalsService, 'openRegularDialog');
  jest.spyOn(mockedSearchContext, 'pauseUserInteraction');

  render(
    <MockAppContextProvider appContext={appContext}>
      <ConnectionsContextProvider>
        <VnetContextProvider>
          <SearchBarConnected />
        </VnetContextProvider>
      </ConnectionsContextProvider>
    </MockAppContextProvider>
  );

  const results = screen.getByRole('menu');
  expect(results).toHaveTextContent(
    'Some of the search results are incomplete.'
  );
  expect(results).toHaveTextContent('Could not fetch resources from foo');
  expect(results).not.toHaveTextContent(resourceSearchError.cause['message']);

  act(() => screen.getByText('Show details').click());

  expect(appContext.modalsService.openRegularDialog).toHaveBeenCalledWith(
    expect.objectContaining({
      kind: 'resource-search-errors',
      errors: [resourceSearchError],
    })
  );
  expect(mockedSearchContext.pauseUserInteraction).toHaveBeenCalled();
});

it('maintains focus on the search input after closing a resource search error modal', async () => {
  const user = userEvent.setup();
  const appContext = setUpContext('/clusters/foo');

  const resourceSearchError = new ResourceSearchError(
    '/clusters/foo',
    new Error('whoops')
  );

  const mockActionAttempts = {
    displayResultsAction,
    filterActions: [],
    resourceActionsAttempt: makeSuccessAttempt([]),
    resourceSearchAttempt: makeSuccessAttempt({
      results: [],
      errors: [resourceSearchError],
      search: '',
    }),
  };
  jest
    .spyOn(useActionAttempts, 'useActionAttempts')
    .mockImplementation(() => mockActionAttempts);

  render(
    <MockAppContextProvider appContext={appContext}>
      <ConnectionsContextProvider>
        <VnetContextProvider>
          <SearchBarConnected />
          <ModalsHost />
        </VnetContextProvider>
      </ConnectionsContextProvider>
    </MockAppContextProvider>
  );

  await act(() => user.type(screen.getByRole('searchbox'), 'foo'));

  expect(screen.getByRole('menu')).toHaveTextContent(
    'Some of the search results are incomplete.'
  );
  act(() => screen.getByText('Show details').click());

  const modal = screen.getByTestId('Modal');
  expect(modal).toHaveTextContent('Resource search errors');
  expect(modal).toHaveTextContent('whoops');

  // Lose focus on the search input.
  act(() => screen.getByText('Close').focus());
  act(() => screen.getByText('Close').click());

  // Need to await this since some state updates in SearchContext are done after the modal closes.
  // Otherwise we'd get a warning about missing `act`.
  await waitFor(() => {
    expect(modal).not.toBeInTheDocument();
  });

  expect(screen.getByRole('searchbox')).toHaveFocus();
  // Verify that the search bar wasn't closed.
  expect(screen.getByRole('menu')).toBeInTheDocument();
});

it('shows a login modal when a request to a cluster from the current workspace fails with a retryable error', async () => {
  const user = userEvent.setup();
  const cluster = makeRootCluster();
  const resourceSearchError = new ResourceSearchError(
    cluster.uri,
    makeRetryableError()
  );
  const resourceSearchResult = {
    results: [],
    errors: [resourceSearchError],
    search: 'foo',
  };
  const resourceSearch = async () => resourceSearchResult;
  jest
    .spyOn(useSearch, 'useResourceSearch')
    .mockImplementation(() => resourceSearch);

  const appContext = setUpContext(cluster.uri);
  appContext.workspacesService.setState(draft => {
    draft.rootClusterUri = cluster.uri;
  });
  appContext.clustersService.setState(draftState => {
    draftState.clusters.set(cluster.uri, cluster);
  });

  render(
    <MockAppContextProvider appContext={appContext}>
      <ConnectionsContextProvider>
        <VnetContextProvider>
          <SearchBarConnected />
          <ModalsHost />
        </VnetContextProvider>
      </ConnectionsContextProvider>
    </MockAppContextProvider>
  );

  await act(() => user.type(screen.getByRole('searchbox'), 'foo'));

  // Verify that the login modal was shown after typing in the search box.
  await waitFor(() => {
    expect(screen.getByTestId('Modal')).toBeInTheDocument();
  });
  expect(screen.getByTestId('Modal')).toHaveTextContent('Login to');

  // Verify that the search bar stays open after closing the modal.
  act(() => screen.getByLabelText('Close').click());
  await waitFor(() => {
    expect(screen.queryByTestId('Modal')).not.toBeInTheDocument();
  });
  expect(screen.getByRole('menu')).toBeInTheDocument();
});

it('closes on a click on an unfocusable element outside of the search bar', async () => {
  const user = userEvent.setup();
  const cluster = makeRootCluster();
  const resourceSearchResult = {
    results: [],
    errors: [],
    search: 'foo',
  };
  const resourceSearch = async () => resourceSearchResult;
  jest
    .spyOn(useSearch, 'useResourceSearch')
    .mockImplementation(() => resourceSearch);

  const appContext = setUpContext(cluster.uri);
  appContext.clustersService.setState(draftState => {
    draftState.clusters.set(cluster.uri, cluster);
  });

  render(
    <MockAppContextProvider appContext={appContext}>
      <ConnectionsContextProvider>
        <VnetContextProvider>
          <SearchBarConnected />
          <p data-testid="unfocusable-element">Lorem ipsum</p>
        </VnetContextProvider>
      </ConnectionsContextProvider>
    </MockAppContextProvider>
  );

  await act(() => user.type(screen.getByRole('searchbox'), 'foo'));
  expect(screen.getByRole('menu')).toBeInTheDocument();

  act(() => {
    screen.getByTestId('unfocusable-element').click();
  });
  expect(screen.queryByRole('menu')).not.toBeInTheDocument();
});

const getMockedSearchContext = (): SearchContext.SearchContext => ({
  inputValue: 'foo',
  filters: [],
  setFilter: () => {},
  removeFilter: () => {},
  isOpen: true,
  open: () => {},
  close: () => {},
  closeWithoutRestoringFocus: () => {},
  resetInput: () => {},
  changeActivePicker: () => {},
  setInputValue: () => {},
  activePicker: pickers.actionPicker,
  inputRef: undefined,
  pauseUserInteraction: async cb => {
    cb();
  },
  addWindowEventListener: () => ({
    cleanup: () => {},
  }),
  makeEventListener: cb => cb,
  advancedSearchEnabled: false,
  toggleAdvancedSearch: () => {},
});

const setUpContext = (clusterUri: ClusterUri) => {
  const appContext = new MockAppContext();
  appContext.workspacesService.setState(draft => {
    draft.rootClusterUri = clusterUri;
    draft.workspaces = {
      [clusterUri]: {
        documents: [],
        location: undefined,
        localClusterUri: clusterUri,
        accessRequests: undefined,
      },
    };
  });
  return appContext;
};
