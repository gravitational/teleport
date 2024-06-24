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

import { getCurrentTheme, getNextTheme } from 'design/ThemeProvider';
import { Theme } from 'gen-proto-ts/teleport/userpreferences/v1/theme_pb';
import { UserPreferences } from 'gen-proto-ts/teleport/userpreferences/v1/userpreferences_pb';

import {
  BackendUserPreferences,
  convertBackendUserPreferences,
  convertUserPreferences,
  isBackendUserPreferences,
} from 'teleport/services/userPreferences/userPreferences';

test('should convert the old cluster user preferences format to the new one', () => {
  // this is how the backend currently returns cluster preferences - as an array of strings
  // instead of the protobuf representation of an object with a `resourceIds` field that contains
  // that array of strings
  const oldBackendPreferences: BackendUserPreferences = {
    theme: Theme.LIGHT,
    clusterPreferences: {
      pinnedResources: ['resource1', 'resource2'],
    },
  };

  const actualUserPreferences: UserPreferences = {
    theme: Theme.LIGHT,
    clusterPreferences: {
      pinnedResources: { resourceIds: ['resource1', 'resource2'] },
    },
  };

  // when we grab the user preferences from the local storage, we check if it is in the old format
  expect(isBackendUserPreferences(oldBackendPreferences)).toBe(true);
  expect(isBackendUserPreferences(actualUserPreferences)).toBe(false);

  // and convert it to the new format if it is
  const newPreferences = convertBackendUserPreferences(oldBackendPreferences);

  expect(newPreferences.clusterPreferences.pinnedResources.resourceIds).toEqual(
    oldBackendPreferences.clusterPreferences.pinnedResources
  );
});

test('should convert the user preferences back to the old format when updating', () => {
  // the backend still expects the old format when updating user preferences

  const actualUserPreferences: UserPreferences = {
    theme: Theme.LIGHT,
    clusterPreferences: {
      pinnedResources: { resourceIds: ['resource1', 'resource2'] },
    },
  };

  const convertedPreferences = convertUserPreferences(actualUserPreferences);

  expect(convertedPreferences.clusterPreferences.pinnedResources).toEqual(
    actualUserPreferences.clusterPreferences.pinnedResources.resourceIds
  );
});

test('getCurrentTheme', () => {
  mockMatchMediaWindow('dark');
  let currentTheme = getCurrentTheme(Theme.UNSPECIFIED);
  expect(currentTheme).toBe(Theme.DARK);

  mockMatchMediaWindow('light');
  currentTheme = getCurrentTheme(Theme.UNSPECIFIED);
  expect(currentTheme).toBe(Theme.LIGHT);

  currentTheme = getCurrentTheme(Theme.LIGHT);
  expect(currentTheme).toBe(Theme.LIGHT);

  currentTheme = getCurrentTheme(Theme.DARK);
  expect(currentTheme).toBe(Theme.DARK);
});

test('getNextTheme', () => {
  mockMatchMediaWindow('dark');
  let nextTheme = getNextTheme(Theme.UNSPECIFIED);
  expect(nextTheme).toBe(Theme.LIGHT);

  mockMatchMediaWindow('light');
  nextTheme = getNextTheme(Theme.UNSPECIFIED);
  expect(nextTheme).toBe(Theme.DARK);

  nextTheme = getNextTheme(Theme.LIGHT);
  expect(nextTheme).toBe(Theme.DARK);

  nextTheme = getNextTheme(Theme.DARK);
  expect(nextTheme).toBe(Theme.LIGHT);
});

function mockMatchMediaWindow(prefers: 'light' | 'dark') {
  return Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: jest.fn().mockImplementation(query => ({
      matches: query === `(prefers-color-scheme: ${prefers})`,
      media: query,
    })),
  });
}
