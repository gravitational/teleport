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

import { PropsWithChildren } from 'react';

import '@testing-library/jest-dom';

import {
  act,
  createEvent,
  fireEvent,
  render,
  renderHook,
  screen,
} from '@testing-library/react';

import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { IAppContext } from 'teleterm/ui/types';

import { SearchContextProvider, useSearchContext } from './SearchContext';

describe('pauseUserInteraction', () => {
  let resolveSuccessAction, rejectFailureAction;
  const successAction = new Promise(resolve => {
    resolveSuccessAction = resolve;
  });
  const failureAction = new Promise((resolve, reject) => {
    rejectFailureAction = reject;
  });

  test.each([
    {
      name: 'prevents the window listeners from being added for the duration of the action',
      action: successAction,
      finishAction: resolveSuccessAction,
    },
    {
      name: 'properly cleans up the state even after the action fails',
      action: failureAction,
      finishAction: rejectFailureAction,
    },
  ])('$name', async ({ action, finishAction }) => {
    const inputFocus = jest.fn();
    const onWindowClick = jest.fn();
    const { result } = renderHook(() => useSearchContext(), {
      wrapper: ({ children }) => <Wrapper>{children}</Wrapper>,
    });
    result.current.inputRef.current = {
      focus: inputFocus,
    } as unknown as HTMLInputElement;

    let pauseInteractionPromise: Promise<void>;
    act(() => {
      pauseInteractionPromise = result.current.pauseUserInteraction(
        () => action
      );
    });

    // Adding a window event while the interaction is paused should be a noop.
    result.current.addWindowEventListener('click', onWindowClick);
    fireEvent(window, createEvent.click(window));
    expect(onWindowClick).not.toHaveBeenCalled();

    await act(async () => {
      finishAction();
      try {
        await pauseInteractionPromise;
      } catch {
        // Ignore the error happening when `finishAction` rejects `action`.
      }
    });

    // User interaction is no longer paused, so addWindowEventListener should add a listener.
    result.current.addWindowEventListener('click', onWindowClick);
    fireEvent(window, createEvent.click(window));
    expect(onWindowClick).toHaveBeenCalledTimes(1);

    // Verify that the search input has received focus after pauseInteractionPromise finishes.
    expect(inputFocus).toHaveBeenCalledTimes(1);
  });
});

describe('addWindowEventListener', () => {
  it('returns a cleanup function', () => {
    const onWindowClick = jest.fn();
    const { result } = renderHook(() => useSearchContext(), {
      wrapper: ({ children }) => <Wrapper>{children}</Wrapper>,
    });

    const { cleanup } = result.current.addWindowEventListener(
      'click',
      onWindowClick,
      // Add an extra arg to make sure that the same set of args is passed to removeEventListener as
      // it is to addEventListener.
      { capture: true }
    );

    fireEvent(window, createEvent.click(window));
    expect(onWindowClick).toHaveBeenCalledTimes(1);

    cleanup();

    fireEvent(window, createEvent.click(window));
    // Verify that the listener was removed by the cleanup function.
    expect(onWindowClick).toHaveBeenCalledTimes(1);
  });

  it('does not return a cleanup function when user interaction is paused', async () => {
    let resolveAction;
    const action = new Promise(resolve => {
      resolveAction = resolve;
    });

    const { result } = renderHook(() => useSearchContext(), {
      wrapper: ({ children }) => <Wrapper>{children}</Wrapper>,
    });

    let pauseInteractionPromise;
    act(() => {
      pauseInteractionPromise = result.current.pauseUserInteraction(
        () => action
      );
    });

    const { cleanup } = result.current.addWindowEventListener(
      'click',
      jest.fn()
    );
    expect(cleanup).toBeUndefined();

    await act(async () => {
      resolveAction();
      await pauseInteractionPromise;
    });
  });
});

describe('open', () => {
  it('manages the focus properly when called with no arguments', () => {
    const SearchInput = () => {
      const { inputRef, isOpen, open, close } = useSearchContext();

      return (
        <>
          <input data-testid="search-input" ref={inputRef} />
          <div data-testid="is-open">{String(isOpen)}</div>
          <button data-testid="open" onClick={() => open()} />
          <button data-testid="close" onClick={() => close()} />
        </>
      );
    };

    render(
      <>
        <input data-testid="other-input" />

        <MockAppContextProvider>
          <SearchContextProvider>
            <SearchInput />
          </SearchContextProvider>
        </MockAppContextProvider>
      </>
    );

    const otherInput = screen.getByTestId('other-input');
    otherInput.focus();

    expect(screen.getByTestId('is-open')).toHaveTextContent('false');
    act(() => screen.getByTestId('open').click());
    expect(screen.getByTestId('is-open')).toHaveTextContent('true');

    act(() => screen.getByTestId('close').click());
    expect(otherInput).toHaveFocus();
  });
});

describe('close', () => {
  it('restores focus on the previously active element', () => {
    const previouslyActive = {
      focus: jest.fn(),
    } as unknown as HTMLInputElement;
    const { result } = renderHook(() => useSearchContext(), {
      wrapper: ({ children }) => <Wrapper>{children}</Wrapper>,
    });

    act(() => {
      result.current.open(previouslyActive);
    });

    act(() => {
      result.current.close();
    });

    expect(previouslyActive.focus).toHaveBeenCalledTimes(1);
  });
});

describe('closeWithoutRestoringFocus', () => {
  it('does not restore focus on the previously active element', () => {
    const previouslyActive = {
      focus: jest.fn(),
    } as unknown as HTMLInputElement;
    const { result } = renderHook(() => useSearchContext(), {
      wrapper: ({ children }) => <Wrapper>{children}</Wrapper>,
    });

    act(() => {
      result.current.open(previouslyActive);
    });

    act(() => {
      result.current.closeWithoutRestoringFocus();
    });

    expect(previouslyActive.focus).not.toHaveBeenCalled();
  });
});

test('search bar state is adjusted to the active document', () => {
  const rootCluster = makeRootCluster({ uri: '/clusters/localhost' });
  const appContext = new MockAppContext();
  appContext.addRootCluster(rootCluster);

  const docService =
    appContext.workspacesService.getActiveWorkspaceDocumentService();
  const { result } = renderHook(() => useSearchContext(), {
    wrapper: ({ children }) => (
      <Wrapper appContext={appContext}>{children}</Wrapper>
    ),
  });

  // initial state, no document
  expect(result.current.inputValue).toBe('');
  expect(result.current.filters).toEqual([]);
  expect(result.current.advancedSearchEnabled).toBe(false);

  // document changes to the cluster document
  act(() => {
    const clusterDoc = docService.createClusterDocument({
      clusterUri: rootCluster.uri,
      queryParams: {
        search: 'foo',
        resourceKinds: ['db'],
        sort: { dir: 'ASC', fieldName: 'name' },
        advancedSearchEnabled: true,
      },
    });
    docService.add(clusterDoc);
    docService.open(clusterDoc.uri);
  });

  expect(result.current.inputValue).toBe('foo');
  expect(result.current.filters).toEqual([
    { filter: 'resource-type', resourceType: 'db' },
  ]);
  expect(result.current.advancedSearchEnabled).toBe(true);

  // document changes to another cluster document
  act(() => {
    const clusterDoc = docService.createClusterDocument({
      clusterUri: rootCluster.uri,
      queryParams: {
        search: 'bar',
        resourceKinds: ['kube_cluster'],
        sort: { dir: 'ASC', fieldName: 'name' },
        advancedSearchEnabled: false,
      },
    });
    docService.add(clusterDoc);
    docService.open(clusterDoc.uri);
  });

  expect(result.current.inputValue).toBe('bar');
  expect(result.current.filters).toEqual([
    { filter: 'resource-type', resourceType: 'kube_cluster' },
  ]);
  expect(result.current.advancedSearchEnabled).toBe(false);

  // document changes to a non-cluster document
  act(() => {
    const clusterDoc = docService.createTshNodeDocument(
      '/clusters/abc/servers/bar',
      { origin: 'search_bar' }
    );
    docService.add(clusterDoc);
    docService.open(clusterDoc.uri);
  });

  expect(result.current.inputValue).toBe('');
  expect(result.current.filters).toEqual([]);
  expect(result.current.advancedSearchEnabled).toBe(false);

  // document changes to a cluster document
  act(() => {
    const clusterDoc = docService.createClusterDocument({
      clusterUri: rootCluster.uri,
      queryParams: {
        search: 'bar',
        resourceKinds: ['kube_cluster'],
        sort: { dir: 'ASC', fieldName: 'name' },
        advancedSearchEnabled: false,
      },
    });
    docService.add(clusterDoc);
    docService.open(clusterDoc.uri);
  });

  expect(result.current.inputValue).toBe('bar');
  expect(result.current.filters).toEqual([
    { filter: 'resource-type', resourceType: 'kube_cluster' },
  ]);
  expect(result.current.advancedSearchEnabled).toBe(false);

  // closing all documents
  act(() => {
    docService.getDocuments().forEach(d => {
      docService.close(d.uri);
    });
  });

  expect(result.current.inputValue).toBe('');
  expect(result.current.filters).toEqual([]);
  expect(result.current.advancedSearchEnabled).toBe(false);
});

function Wrapper(props: PropsWithChildren<{ appContext?: IAppContext }>) {
  return (
    <MockAppContextProvider appContext={props.appContext}>
      <SearchContextProvider>{props.children}</SearchContextProvider>
    </MockAppContextProvider>
  );
}
