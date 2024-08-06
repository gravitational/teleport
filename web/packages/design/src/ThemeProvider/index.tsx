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
  WebTarget,
} from 'styled-components';

import { KeysEnum, storageService } from 'teleport/services/storageService';

import cfg from 'teleport/config';

import { Theme } from 'gen-proto-ts/teleport/userpreferences/v1/theme_pb';

import isPropValid from '@emotion/is-prop-valid';

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
      <StyleSheetManager shouldForwardProp={shouldForwardProp}>
        <React.Fragment>
          <GlobalStyle />
          {props.children}
        </React.Fragment>
      </StyleSheetManager>
    </StyledThemeProvider>
  );
};

/**
 * This function has been taken from the [styled-components library
 * FAQ](https://styled-components.com/docs/faqs#shouldforwardprop-is-no-longer-provided-by-default).
 * It implements the default behavior from styled-components v5. It's required,
 * because it would be otherwise incompatible with styled-system (or at least
 * the way we are using it). Not using this function would cause a lot of props
 * being passed unnecessarily to the underlying elements. Not only it's
 * unnecessary and potentially a buggy behavior, it also causes a lot of
 * warnings printed on the console, which in turn causes test failures.
 */
export function shouldForwardProp(propName: string, target: WebTarget) {
  if (typeof target === 'string') {
    // For HTML elements, forward the prop if it is a valid HTML attribute
    return isPropValid(propName);
  }
  // For other elements, forward all props
  return true;
}

export default ThemeProvider;
