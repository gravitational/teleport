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

import { wait } from 'shared/utils/wait';
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

  expect(result.current.userPreferencesAttempt).toEqual({
    status: 'success',
    data: preferences,
    statusText: '',
  });

  // updating the fallback
  expect(
    appContext.workspacesService.setUnifiedResourcePreferences
  ).toHaveBeenCalledWith(cluster.uri, preferences.unifiedResourcePreferences);
});

test('unified resources fallback preferences are taken from a workspace', async () => {
  const appContext = new MockAppContext();

  const getUserPreferencesPromise = Promise.resolve({});
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

  await act(() => getUserPreferencesPromise);

  expect(result.current.unifiedResourcePreferencesFallback).toEqual(
    preferences.unifiedResourcePreferences
  );
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

    expect(result.current.updateUserPreferencesAttempt.status).toBe('success');
    expect(appContext.tshd.updateUserPreferences).toHaveBeenCalledWith({
      clusterUri: cluster.uri,
      userPreferences: newPreferences,
    });
    expect(
      result.current.userPreferencesAttempt.status === 'success' &&
        result.current.userPreferencesAttempt.data
    ).toEqual(newPreferences);
  });

  test('when the request for fetching preferences is in-flight', async () => {
    let rejectGetUserPreferencesPromise: () => void;
    const getUserPreferencesPromise = new Promise((resolve, reject) => {
      rejectGetUserPreferencesPromise = reject;
    });

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

    const newPreferences: UserPreferences = {
      clusterPreferences: {},
      unifiedResourcePreferences: {
        viewMode: ViewMode.VIEW_MODE_LIST,
        defaultTab: DefaultTab.DEFAULT_TAB_PINNED,
      },
    };

    await act(() => result.current.updateUserPreferences(newPreferences));

    expect(result.current.updateUserPreferencesAttempt.status).toBe('success');
    expect(appContext.tshd.updateUserPreferences).toHaveBeenCalledWith({
      clusterUri: cluster.uri,
      userPreferences: newPreferences,
    });
    expect(
      result.current.userPreferencesAttempt.status === 'success' &&
        result.current.userPreferencesAttempt.data
    ).toEqual(newPreferences);

    // updating the fallback
    expect(
      appContext.workspacesService.setUnifiedResourcePreferences
    ).toHaveBeenCalledWith(
      cluster.uri,
      newPreferences.unifiedResourcePreferences
    );
    rejectGetUserPreferencesPromise();
  });

  test('when the request for fetching preferences is in-flight and the request to update them fails', async () => {
    // resolves after 100 ms, after the request to update the preference returns
    const getUserPreferencesPromise = wait(100).then(() => preferences);

    jest
      .spyOn(appContext.tshd, 'getUserPreferences')
      .mockImplementation(() => getUserPreferencesPromise);
    jest
      .spyOn(appContext.tshd, 'updateUserPreferences')
      .mockRejectedValue('Failed to update');

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

    await act(() => result.current.updateUserPreferences(newPreferences));
    await act(() => getUserPreferencesPromise);

    expect(
      result.current.updateUserPreferencesAttempt.status === 'error' &&
        result.current.updateUserPreferencesAttempt.error
    ).toBe('Failed to update');

    // updating the fallback
    expect(
      appContext.workspacesService.setUnifiedResourcePreferences
    ).toHaveBeenCalledWith(
      cluster.uri,
      newPreferences.unifiedResourcePreferences // the fallback is left with new preferences
    );

    expect(
      result.current.userPreferencesAttempt.status === 'success' &&
        result.current.userPreferencesAttempt.data
    ).toEqual(preferences); // non-updated preferences
  });
});
