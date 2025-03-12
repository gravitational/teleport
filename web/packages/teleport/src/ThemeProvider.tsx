/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { ReactNode, useEffect, useState } from 'react';

import { bblpTheme, darkTheme, lightTheme, Theme } from 'design/theme';
import { ConfiguredThemeProvider } from 'design/ThemeProvider';
import { Theme as ThemePreference } from 'gen-proto-ts/teleport/userpreferences/v1/theme_pb';

import cfg from 'teleport/config';
import { KeysEnum, storageService } from 'teleport/services/storageService';

const customThemes = {
  bblp: bblpTheme,
};

export const ThemeProvider = (props: { children?: ReactNode }) => {
  const [themePreference, setThemePreference] = useState<ThemePreference>(
    storageService.getThemePreference()
  );

  useEffect(() => {
    storageService.subscribe(receiveMessage);

    function receiveMessage(event: StorageEvent) {
      const { key, newValue } = event;

      if (!newValue || key !== KeysEnum.USER_PREFERENCES) {
        return;
      }

      const preferences = JSON.parse(newValue);
      if (
        preferences.theme !== ThemePreference.UNSPECIFIED &&
        preferences.theme !== themePreference
      ) {
        setThemePreference(preferences.theme);
      }
    }

    // Cleanup on unmount
    return function unsubscribe() {
      storageService.unsubscribe(receiveMessage);
    };
  }, [themePreference]);

  let theme = themePreferenceToTheme(themePreference);
  if (customThemes[cfg.customTheme]) {
    theme = customThemes[cfg.customTheme];
  }

  return (
    <ConfiguredThemeProvider theme={theme}>
      {props.children}
    </ConfiguredThemeProvider>
  );
};

function themePreferenceToTheme(themePreference: ThemePreference): Theme {
  if (themePreference === ThemePreference.UNSPECIFIED) {
    return getPrefersDark() ? lightTheme : darkTheme;
  }
  return themePreference === ThemePreference.LIGHT ? lightTheme : darkTheme;
}

/**
 * Determines the current theme preference.
 *
 * If the provided `currentTheme` is `UNSPECIFIED`, it checks the user's
 * system preference and returns a theme based on it.
 *
 * @TODO(avatus) when we add user settings page, we can add a Theme.SYSTEM option
 * and remove the checks for unspecified
 */
export function getCurrentTheme(
  currentTheme: ThemePreference
): ThemePreference {
  if (currentTheme === ThemePreference.UNSPECIFIED) {
    return getPrefersDark() ? ThemePreference.DARK : ThemePreference.LIGHT;
  }

  return currentTheme;
}

export function getNextTheme(currentTheme: ThemePreference): ThemePreference {
  return getCurrentTheme(currentTheme) === ThemePreference.LIGHT
    ? ThemePreference.DARK
    : ThemePreference.LIGHT;
}

export function getPrefersDark(): boolean {
  return (
    window.matchMedia &&
    window.matchMedia('(prefers-color-scheme: dark)').matches
  );
}

export function updateFavicon() {
  let base = '/web/app/';
  if (import.meta.env.MODE === 'development') {
    base = '/app/';
  }
  const darkModePreferred = getPrefersDark();
  const favicon = document.querySelector('link[rel="icon"]');

  if (favicon instanceof HTMLLinkElement) {
    if (darkModePreferred) {
      favicon.href = base + 'favicon-dark.png';
    } else {
      favicon.href = base + 'favicon-light.png';
    }
  }
}
