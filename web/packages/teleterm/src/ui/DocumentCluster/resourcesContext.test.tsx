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
import { renderHook } from '@testing-library/react-hooks';

import {
  ResourcesContextProvider,
  useResourcesContext,
} from './resourcesContext';

describe('requestResourcesRefresh', () => {
  it('calls listener registered with onResourcesRefreshRequest', () => {
    const wrapper = ({ children }) => (
      <ResourcesContextProvider>{children}</ResourcesContextProvider>
    );
    const { result } = renderHook(() => useResourcesContext(), { wrapper });

    const listener = jest.fn();
    result.current.onResourcesRefreshRequest(listener);
    result.current.requestResourcesRefresh();

    expect(listener).toHaveBeenCalledTimes(1);
  });
});

describe('onResourcesRefreshRequest cleanup function', () => {
  it('removes the listener', () => {
    const wrapper = ({ children }) => (
      <ResourcesContextProvider>{children}</ResourcesContextProvider>
    );
    const { result } = renderHook(() => useResourcesContext(), { wrapper });

    const listener = jest.fn();
    const { cleanup } = result.current.onResourcesRefreshRequest(listener);

    cleanup();
    result.current.requestResourcesRefresh();

    expect(listener).not.toHaveBeenCalled();
  });
});
