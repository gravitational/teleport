/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { DeprecatedThemeOption } from 'design/theme';

import type { AssistUserPreferences } from 'teleport/Assist/types';

export enum ThemePreference {
  Light = 1,
  Dark = 2,
}

export interface UserPreferences {
  theme: ThemePreference;
  assist: AssistUserPreferences;
}

export type UserPreferencesSubset = Subset<UserPreferences>;
export type GetUserPreferencesResponse = UserPreferences;

export function deprecatedThemeToThemePreference(
  theme: DeprecatedThemeOption
): ThemePreference {
  switch (theme) {
    case 'light':
      return ThemePreference.Light;
    case 'dark':
      return ThemePreference.Dark;
  }
}
