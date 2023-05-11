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

import React, { useState, useEffect } from 'react';
import {
  ThemeProvider as StyledThemeProvider,
  StyleSheetManager,
} from 'styled-components';

import storage, { KeysEnum } from 'teleport/services/localStorage';

import { darkTheme, lightTheme } from '../theme';

import { GlobalStyle } from './globals';

import type { ThemeOption } from '../theme';

const ThemeProvider = props => {
  useEffect(() => {
    storage.subscribe(receiveMessage);

    function receiveMessage(event) {
      const { key, newValue } = event;
      if (key === KeysEnum.THEME && newValue) {
        setThemeOption(newValue);
      }
    }

    // Cleanup on unmount
    return function unsubscribe() {
      storage.unsubscribe(receiveMessage);
    };
  }, []);

  const [themeOption, setThemeOption] = useState<ThemeOption>(
    storage.getThemeOption()
  );

  const theme = themeOption === 'dark' ? darkTheme : lightTheme;

  return (
    <StyledThemeProvider theme={props.theme || theme}>
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
