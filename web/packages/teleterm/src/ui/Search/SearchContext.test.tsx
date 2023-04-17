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
import { render, screen } from '@testing-library/react';
import { renderHook, act } from '@testing-library/react-hooks';

import { SearchContextProvider, useSearchContext } from './SearchContext';

describe('lockOpen', () => {
  let resolveSuccessAction, rejectFailureAction;
  const successAction = new Promise(resolve => {
    resolveSuccessAction = resolve;
  });
  const failureAction = new Promise((resolve, reject) => {
    rejectFailureAction = reject;
  });

  test.each([
    {
      name: 'prevents the search bar from being closed for the duration of the action',
      action: successAction,
      finishAction: resolveSuccessAction,
    },
    {
      name: 'properly cleans up the ref even after the action fails',
      action: failureAction,
      finishAction: rejectFailureAction,
    },
  ])('$name', async ({ action, finishAction }) => {
    const inputFocus = jest.fn();
    const { result } = renderHook(() => useSearchContext(), {
      wrapper: ({ children }) => (
        <SearchContextProvider>{children}</SearchContextProvider>
      ),
    });
    result.current.inputRef.current = {
      focus: inputFocus,
    } as unknown as HTMLInputElement;

    act(() => {
      result.current.open();
    });

    let lockOpenPromise: Promise<void>;
    act(() => {
      lockOpenPromise = result.current.lockOpen(action);
    });

    // Closing while the search bar is locked open should be a noop.
    act(() => {
      result.current.close();
    });
    expect(result.current.isOpen).toBe(true);

    await act(async () => {
      finishAction();
      try {
        await lockOpenPromise;
      } catch {
        // Ignore the error happening when `finishAction` rejects `action`.
      }
    });

    // The search bar should be no longer locked open, so close should behave as expected.
    act(() => {
      result.current.close();
    });
    expect(result.current.isOpen).toBe(false);

    // Called the first time during `open`, then again after `lockOpen` finishes.
    expect(inputFocus).toHaveBeenCalledTimes(2);
  });
});

describe('open', () => {
  it('manages the focus properly when called with no arguments', () => {
    const SearchInput = () => {
      const { inputRef, open, close } = useSearchContext();

      return (
        <>
          <input data-testid="search-input" ref={inputRef} />
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
    const searchInput = screen.getByTestId('search-input');
    otherInput.focus();

    screen.getByTestId('open').click();
    expect(searchInput).toHaveFocus();

    screen.getByTestId('close').click();
    expect(otherInput).toHaveFocus();
  });
});
