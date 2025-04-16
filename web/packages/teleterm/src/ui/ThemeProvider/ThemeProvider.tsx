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
  ThemeProvider as StyledThemeProvider,
  StyleSheetManager,
} from 'styled-components';

import { Theme } from 'design/theme/themes/types';
import { shouldForwardProp } from 'design/ThemeProvider';

import { useAppContext } from 'teleterm/ui/appContextProvider';

import { GlobalStyle } from './globals';
import { darkTheme, lightTheme } from './theme';

export const ThemeProvider = (props: React.PropsWithChildren<unknown>) => {
  // Listening to Electron's nativeTheme.on('updated') is a workaround.
  // The renderer should be able to get the current theme via "prefers-color-scheme" media query.
  // Unfortunately, it does not work correctly on Ubuntu where the query from above always returns the old value
  // (for example, when the app was launched in a dark mode, it always returns 'dark'
  // ignoring that the system theme is now 'light').
  // Related Electron issue: https://github.com/electron/electron/issues/21427#issuecomment-589796481,
  // Related Chromium issue: https://bugs.chromium.org/p/chromium/issues/detail?id=998903
  //
  // Additional issue is that nativeTheme does not return correct values at all on Fedora:
  // https://github.com/electron/electron/issues/33635#issuecomment-1502215450
  const ctx = useAppContext();
  const [activeTheme, setActiveTheme] = useState(() =>
    ctx.mainProcessClient.shouldUseDarkColors() ? darkTheme : lightTheme
  );

  useEffect(() => {
    const { cleanup } = ctx.mainProcessClient.subscribeToNativeThemeUpdate(
      ({ shouldUseDarkColors }) => {
        setActiveTheme(shouldUseDarkColors ? darkTheme : lightTheme);
      }
    );

    return cleanup;
  }, [ctx.mainProcessClient]);

  return (
    <StaticThemeProvider theme={activeTheme}>
      {props.children}
    </StaticThemeProvider>
  );
};

/** Uses a theme from a prop. Useful in storybook. */
export const StaticThemeProvider = (
  props: React.PropsWithChildren<{ theme?: Theme }>
) => {
  return (
    <StyledThemeProvider theme={props.theme}>
      <StyleSheetManager shouldForwardProp={shouldForwardProp}>
        <React.Fragment>
          <GlobalStyle />
          {props.children}
        </React.Fragment>
      </StyleSheetManager>
    </StyledThemeProvider>
  );
};
