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
import { renderHook, act } from '@testing-library/react-hooks';

import { SearchContextProvider, useSearchContext } from './SearchContext';

describe('lockOpen', () => {
  it('prevents the search bar from being closed for the duration of the action', async () => {
    const { result } = renderHook(() => useSearchContext(), {
      wrapper: ({ children }) => (
        <SearchContextProvider>{children}</SearchContextProvider>
      ),
    });
    act(() => {
      result.current.open();
    });

    let resolveAction: (value?: unknown) => void;
    const action = new Promise(resolve => {
      resolveAction = resolve;
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
      resolveAction();
      await lockOpenPromise;
    });

    // The search bar should be no longer locked open, so close should behave as expected.
    act(() => {
      result.current.close();
    });
    expect(result.current.isOpen).toBe(false);
  });

  it('prevents the search bar from being closed even when the action rejects', async () => {
    const { result } = renderHook(() => useSearchContext(), {
      wrapper: ({ children }) => (
        <SearchContextProvider>{children}</SearchContextProvider>
      ),
    });
    act(() => {
      result.current.open();
    });

    let rejectAction: () => void;
    const action = new Promise((resolve, reject) => {
      rejectAction = reject;
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
      rejectAction();
      try {
        await lockOpenPromise;
      } catch {
        // Ignore the error.
      }
    });

    // The search bar should be no longer locked open, so close should behave as expected.
    act(() => {
      result.current.close();
    });
    expect(result.current.isOpen).toBe(false);
  });
});
