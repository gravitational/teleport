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

import { renderHook, act } from '@testing-library/react';

import {
  DefaultTab,
  LabelsViewMode,
  ViewMode,
} from 'gen-proto-ts/teleport/userpreferences/v1/unified_resource_preferences_pb';

import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';

import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { UserPreferences } from 'teleterm/services/tshd/types';

import { useUserPreferences } from './useUserPreferences';

const cluster = makeRootCluster();
const preferences: UserPreferences = {
  clusterPreferences: { pinnedResources: { resourceIds: ['abc'] } },
  unifiedResourcePreferences: {
    viewMode: ViewMode.CARD,
    defaultTab: DefaultTab.ALL,
    labelsViewMode: LabelsViewMode.COLLAPSED,
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

describe('updating preferences', () => {
  const appContext = new MockAppContext();
  beforeEach(() => {
    jest
      .spyOn(appContext.workspacesService, 'getUnifiedResourcePreferences')
      .mockReturnValue(undefined);
    jest
      .spyOn(appContext.workspacesService, 'setUnifiedResourcePreferences')
      .mockImplementation();
  });

  it('works correctly when the initial preferences were fetched', async () => {
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
        viewMode: ViewMode.LIST,
        defaultTab: DefaultTab.PINNED,
        labelsViewMode: LabelsViewMode.COLLAPSED,
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

  it('works correctly when the initial preferences have not been fetched yet', async () => {
    let rejectGetUserPreferencesPromise: (error: Error) => void;
    const getUserPreferencesPromise = new Promise<UserPreferences>(
      (resolve, reject) => {
        rejectGetUserPreferencesPromise = reject;
      }
    );
    let resolveUpdateUserPreferencesPromise: (u: UserPreferences) => void;
    const updateUserPreferencesPromise = new Promise(resolve => {
      resolveUpdateUserPreferencesPromise = resolve;
    });

    jest
      .spyOn(appContext.tshd, 'getUserPreferences')
      .mockImplementation((requestParams, abortSignal) => {
        abortSignal.addEventListener(() =>
          rejectGetUserPreferencesPromise(new Error('Aborted'))
        );
        return getUserPreferencesPromise;
      });
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
        viewMode: ViewMode.LIST,
        defaultTab: DefaultTab.PINNED,
        labelsViewMode: LabelsViewMode.COLLAPSED,
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

    // suddenly, the request returns other preferences than what we wanted
    // (e.g., because they were changed it in the browser in the meantime)
    act(() =>
      resolveUpdateUserPreferencesPromise({
        clusterPreferences: { pinnedResources: { resourceIds: ['abc'] } },
        unifiedResourcePreferences: {
          viewMode: ViewMode.CARD,
          defaultTab: DefaultTab.PINNED,
          labelsViewMode: LabelsViewMode.COLLAPSED,
        },
      })
    );
    await act(() => updateUserPreferencesPromise);

    // but our view preferences are still the same as what we sent in the update request!
    expect(result.current.userPreferences.unifiedResourcePreferences).toEqual(
      newPreferences.unifiedResourcePreferences
    );
    expect(
      result.current.userPreferences.clusterPreferences.pinnedResources
        .resourceIds
    ).toEqual(['abc']);
  });
});
