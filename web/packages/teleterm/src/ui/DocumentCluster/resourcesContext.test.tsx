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
import { renderHook } from '@testing-library/react';

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
