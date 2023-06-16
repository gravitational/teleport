/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React, { useEffect, useState } from 'react';
import {
  ThemeProvider as StyledThemeProvider,
  StyleSheetManager,
} from 'styled-components';

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

/** Uses a theme from a prop. Useful in storybbok. */
export const StaticThemeProvider = (
  props: React.PropsWithChildren<{ theme?: unknown }>
) => {
  return (
    <StyledThemeProvider theme={props.theme}>
      <StyleSheetManager disableVendorPrefixes>
        <React.Fragment>
          <GlobalStyle />
          {props.children}
        </React.Fragment>
      </StyleSheetManager>
    </StyledThemeProvider>
  );
};
