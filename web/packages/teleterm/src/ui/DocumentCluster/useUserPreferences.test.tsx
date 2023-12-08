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

import { renderHook, act } from '@testing-library/react';
import {
  ViewMode,
  DefaultTab,
} from 'shared/services/unifiedResourcePreferences';

import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';

import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { UserPreferences } from 'teleterm/services/tshd/types';

import { useUserPreferences } from './useUserPreferences';

const cluster = makeRootCluster();
const preferences: UserPreferences = {
  clusterPreferences: { pinnedResources: { resourceIdsList: ['abc'] } },
  unifiedResourcePreferences: {
    viewMode: ViewMode.VIEW_MODE_CARD,
    defaultTab: DefaultTab.DEFAULT_TAB_ALL,
  },
};

test('user preferences are fetched', async () => {
  const appContext = new MockAppContext();
  const getUserPreferencesPromise = Promise.resolve(preferences);

  jest
    .spyOn(appContext.tshd, 'getUserPreferences')
    .mockImplementation(() => getUserPreferencesPromise);
  jest
    .spyOn(appContext.workspacesService, 'getUnifiedResourcePreferences')
    .mockReturnValue(undefined);
  jest
    .spyOn(appContext.workspacesService, 'setUnifiedResourcePreferences')
    .mockImplementation();

  const { result } = renderHook(() => useUserPreferences(cluster.uri), {
    wrapper: ({ children }) => (
      <MockAppContextProvider appContext={appContext}>
        {children}
      </MockAppContextProvider>
    ),
  });

  await act(() => getUserPreferencesPromise);

  expect(result.current.userPreferences).toEqual(preferences);
  expect(result.current.userPreferencesAttempt.status).toBe('success');

  // updating the fallback
  expect(
    appContext.workspacesService.setUnifiedResourcePreferences
  ).toHaveBeenCalledWith(cluster.uri, preferences.unifiedResourcePreferences);
});

test('unified resources fallback preferences are taken from a workspace', async () => {
  const appContext = new MockAppContext();
  let resolveGetUserPreferencesPromise: (u: UserPreferences) => void;
  const getUserPreferencesPromise = new Promise(resolve => {
    resolveGetUserPreferencesPromise = resolve;
  });

  jest
    .spyOn(appContext.tshd, 'getUserPreferences')
    .mockImplementation(() => getUserPreferencesPromise);
  jest
    .spyOn(appContext.workspacesService, 'getUnifiedResourcePreferences')
    .mockReturnValue(preferences.unifiedResourcePreferences);
  jest
    .spyOn(appContext.workspacesService, 'setUnifiedResourcePreferences')
    .mockImplementation();

  const { result } = renderHook(() => useUserPreferences(cluster.uri), {
    wrapper: ({ children }) => (
      <MockAppContextProvider appContext={appContext}>
        {children}
      </MockAppContextProvider>
    ),
  });

  expect(result.current.userPreferences.unifiedResourcePreferences).toEqual(
    preferences.unifiedResourcePreferences
  );
  resolveGetUserPreferencesPromise(null);
  await act(() => getUserPreferencesPromise);
});

describe('updating preferences works correctly', () => {
  const appContext = new MockAppContext();
  beforeEach(() => {
    jest
      .spyOn(appContext.workspacesService, 'getUnifiedResourcePreferences')
      .mockReturnValue(undefined);
    jest
      .spyOn(appContext.workspacesService, 'setUnifiedResourcePreferences')
      .mockImplementation();
  });

  test('when the preferences are already fetched', async () => {
    const getUserPreferencesPromise = Promise.resolve(preferences);

    jest
      .spyOn(appContext.tshd, 'getUserPreferences')
      .mockImplementation(() => getUserPreferencesPromise);
    jest
      .spyOn(appContext.tshd, 'updateUserPreferences')
      .mockImplementation(async preferences => preferences.userPreferences);

    const { result } = renderHook(() => useUserPreferences(cluster.uri), {
      wrapper: ({ children }) => (
        <MockAppContextProvider appContext={appContext}>
          {children}
        </MockAppContextProvider>
      ),
    });

    await act(() => getUserPreferencesPromise);

    const newPreferences: UserPreferences = {
      clusterPreferences: {},
      unifiedResourcePreferences: {
        viewMode: ViewMode.VIEW_MODE_LIST,
        defaultTab: DefaultTab.DEFAULT_TAB_PINNED,
      },
    };

    await act(() => result.current.updateUserPreferences(newPreferences));

    // updating state
    expect(
      appContext.workspacesService.setUnifiedResourcePreferences
    ).toHaveBeenCalledWith(
      cluster.uri,
      newPreferences.unifiedResourcePreferences
    );
    expect(result.current.userPreferences.unifiedResourcePreferences).toEqual(
      newPreferences.unifiedResourcePreferences
    );

    expect(result.current.userPreferencesAttempt.status).toBe('success');
    expect(appContext.tshd.updateUserPreferences).toHaveBeenCalledWith({
      clusterUri: cluster.uri,
      userPreferences: newPreferences,
    });
  });

  test('when the request for fetching preferences is in-flight', async () => {
    let resolveGetUserPreferencesPromise: (u: UserPreferences) => void;
    const getUserPreferencesPromise = new Promise<UserPreferences>(resolve => {
      resolveGetUserPreferencesPromise = resolve;
    });
    let resolveUpdateUserPreferencesPromise: (u: UserPreferences) => void;
    const updateUserPreferencesPromise = new Promise(resolve => {
      resolveUpdateUserPreferencesPromise = resolve;
    });

    jest
      .spyOn(appContext.tshd, 'getUserPreferences')
      .mockImplementation(() => getUserPreferencesPromise);
    jest
      .spyOn(appContext.tshd, 'updateUserPreferences')
      .mockImplementation(() => updateUserPreferencesPromise);

    const { result } = renderHook(() => useUserPreferences(cluster.uri), {
      wrapper: ({ children }) => (
        <MockAppContextProvider appContext={appContext}>
          {children}
        </MockAppContextProvider>
      ),
    });

    const newPreferences: UserPreferences = {
      clusterPreferences: {},
      unifiedResourcePreferences: {
        viewMode: ViewMode.VIEW_MODE_LIST,
        defaultTab: DefaultTab.DEFAULT_TAB_PINNED,
      },
    };

    act(() => {
      result.current.updateUserPreferences(newPreferences);
    });

    // updating state
    expect(
      appContext.workspacesService.setUnifiedResourcePreferences
    ).toHaveBeenCalledWith(
      cluster.uri,
      newPreferences.unifiedResourcePreferences
    );
    expect(result.current.userPreferences.unifiedResourcePreferences).toEqual(
      newPreferences.unifiedResourcePreferences
    );

    expect(result.current.userPreferencesAttempt.status).toBe('processing');
    expect(appContext.tshd.updateUserPreferences).toHaveBeenCalledWith({
      clusterUri: cluster.uri,
      userPreferences: newPreferences,
    });

    act(() => resolveGetUserPreferencesPromise(null));
    await act(() => getUserPreferencesPromise);
    act(() => resolveUpdateUserPreferencesPromise(null));
    await act(() => updateUserPreferencesPromise);
  });
});
