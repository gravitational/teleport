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

import React, { useEffect, useState } from 'react';
import {
  StyleSheetManager,
  ThemeProvider as StyledThemeProvider,
} from 'styled-components';

import { KeysEnum, storageService } from 'teleport/services/storageService';

import cfg from 'teleport/config';

import { Theme } from 'gen-proto-ts/teleport/userpreferences/v1/theme_pb';

import { darkTheme, lightTheme, bblpTheme } from '../theme';

import { GlobalStyle } from './globals';

function themePreferenceToTheme(themePreference: Theme) {
  if (themePreference === Theme.UNSPECIFIED) {
    return getPrefersDark() ? lightTheme : darkTheme;
  }
  return themePreference === Theme.LIGHT ? lightTheme : darkTheme;
}

// because unspecified can exist but only used as a fallback and not an option,
// we need to get the current/next themes with getPrefersDark in mind.
// TODO (avatus) when we add user settings page, we can add a Theme.SYSTEM option
// and remove the checks for unspecified
export function getCurrentTheme(currentTheme: Theme): Theme {
  if (currentTheme === Theme.UNSPECIFIED) {
    return getPrefersDark() ? Theme.DARK : Theme.LIGHT;
  }

  return currentTheme;
}

export function getNextTheme(currentTheme: Theme): Theme {
  return getCurrentTheme(currentTheme) === Theme.LIGHT
    ? Theme.DARK
    : Theme.LIGHT;
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

const ThemeProvider = props => {
  const [themePreference, setThemePreference] = useState<Theme>(
    storageService.getThemePreference()
  );

  useEffect(() => {
    storageService.subscribe(receiveMessage);

    function receiveMessage(event) {
      const { key, newValue } = event;

      if (!newValue || key !== KeysEnum.USER_PREFERENCES) {
        return;
      }

      const preferences = JSON.parse(newValue);
      if (
        preferences.theme !== Theme.UNSPECIFIED &&
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

  const customThemes = {
    bblp: bblpTheme,
  };

  // If no props.theme is defined, use the custom theme instead of the user preference theme.
  let theme;
  if (props.theme) {
    theme = props.theme;
  } else if (customThemes[cfg.customTheme]) {
    theme = customThemes[cfg.customTheme];
  } else {
    theme = themePreferenceToTheme(themePreference);
  }

  return (
    <StyledThemeProvider theme={theme}>
      <StyleSheetManager disableVendorPrefixes>
        <React.Fragment>
          <GlobalStyle />
          {props.children}
        </React.Fragment>
      </StyleSheetManager>
    </StyledThemeProvider>
  );
};

export default ThemeProvider;
