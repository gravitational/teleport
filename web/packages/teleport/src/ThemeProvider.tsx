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

import { useEffect, useMemo, useState, type PropsWithChildren } from 'react';

import {
  createThemeSystem,
  LEGACY_THEME_COLORS,
  ThemeProvider as NewThemeProvider,
  TELEPORT_THEME,
  THEMES,
  UiThemeMode,
} from '@gravitational/design-system';
import {
  bblpTheme,
  darkTheme,
  lightTheme,
  Theme,
  type ThemeDefinition,
} from 'design/theme';
import { sharedColors } from 'design/theme/themes/sharedStyles';
import { ConfiguredThemeProvider } from 'design/ThemeProvider';
import { Theme as ThemePreference } from 'gen-proto-ts/teleport/userpreferences/v1/theme_pb';

import cfg from 'teleport/config';
import { KeysEnum, storageService } from 'teleport/services/storageService';

const customThemes = {
  bblp: bblpTheme,
  // Lock mc to light theme, and flag it as a custom theme to disable the theme switcher.
  mc: { ...lightTheme, isCustomTheme: true },
};

export function ThemeProvider({ children }: PropsWithChildren) {
  const themePreference = useThemePreference();

  const selectedTheme = useMemo(() => {
    const theme =
      THEMES.find(t => t.name === cfg.customTheme) ??
      THEMES.find(t => t.name === TELEPORT_THEME.name);

    return {
      ...theme,
      system: createThemeSystem(theme.config),
    };
  }, []);

  const colorMode = useMemo(() => {
    switch (selectedTheme.mode) {
      case UiThemeMode.SingleColor:
        return;

      case UiThemeMode.ForcedColor:
        return selectedTheme.forcedColorMode;

      case UiThemeMode.LightAndDark:
        if (themePreference === ThemePreference.UNSPECIFIED) {
          return getPrefersDark() ? 'light' : 'dark';
        }

        return themePreference === ThemePreference.LIGHT ? 'light' : 'dark';
    }
  }, [themePreference, selectedTheme]);

  const legacyTheme: Theme = useMemo(() => {
    let theme = themePreferenceToTheme(themePreference);

    if (customThemes[cfg.customTheme]) {
      theme = customThemes[cfg.customTheme];
    }

    return {
      ...theme,
      colors: {
        ...sharedColors,
        ...LEGACY_THEME_COLORS,
      },
    };
  }, [themePreference]);

  return (
    <NewThemeProvider forcedTheme={colorMode} system={selectedTheme.system}>
      <ConfiguredThemeProvider theme={legacyTheme}>
        {children}
      </ConfiguredThemeProvider>
    </NewThemeProvider>
  );
}

function useThemePreference() {
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

  return themePreference;
}

function themePreferenceToTheme(
  themePreference: ThemePreference
): ThemeDefinition {
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
