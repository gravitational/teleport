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
import '@testing-library/jest-dom';
import { fireEvent, createEvent, render, screen } from '@testing-library/react';
import { renderHook, act } from '@testing-library/react-hooks';

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
      wrapper: ({ children }) => (
        <SearchContextProvider>{children}</SearchContextProvider>
      ),
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
      wrapper: ({ children }) => (
        <SearchContextProvider>{children}</SearchContextProvider>
      ),
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
      wrapper: ({ children }) => (
        <SearchContextProvider>{children}</SearchContextProvider>
      ),
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

        <SearchContextProvider>
          <SearchInput />
        </SearchContextProvider>
      </>
    );

    const otherInput = screen.getByTestId('other-input');
    otherInput.focus();

    expect(screen.getByTestId('is-open')).toHaveTextContent('false');
    screen.getByTestId('open').click();
    expect(screen.getByTestId('is-open')).toHaveTextContent('true');

    screen.getByTestId('close').click();
    expect(otherInput).toHaveFocus();
  });
});

describe('close', () => {
  it('restores focus on the previously active element', () => {
    const previouslyActive = {
      focus: jest.fn(),
    } as unknown as HTMLInputElement;
    const { result } = renderHook(() => useSearchContext(), {
      wrapper: ({ children }) => (
        <SearchContextProvider>{children}</SearchContextProvider>
      ),
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
      wrapper: ({ children }) => (
        <SearchContextProvider>{children}</SearchContextProvider>
      ),
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
