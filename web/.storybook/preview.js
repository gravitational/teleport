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

import React from 'react';
import { setupWorker, rest } from 'msw';
import { addDecorator, addParameters } from '@storybook/react';
import { darkTheme, lightTheme } from './../packages/design/src/theme';
import DefaultThemeProvider from '../packages/design/src/ThemeProvider';
import Box from './../packages/design/src/Box';
import '../packages/teleport/src/lib/polyfillRandomUuid';
import { StaticThemeProvider as TeletermThemeProvider } from './../packages/teleterm/src/ui/ThemeProvider';
import {
  darkTheme as teletermDarkTheme,
  lightTheme as teletermLightTheme,
} from './../packages/teleterm/src/ui/ThemeProvider/theme';
import { handlersTeleport } from './../packages/teleport/src/mocks/handlers';

// Checks we are running non-node environment (browser)
if (typeof global.process === 'undefined') {
  const worker = setupWorker(...handlersTeleport);
  worker.start();

  // So it can be accessed in stories more easily.
  window.msw = { worker, rest };
}

// wrap each story with theme provider
const ThemeDecorator = (storyFn, meta) => {
  let ThemeProvider;
  let theme;

  if (meta.title.startsWith('Teleterm/')) {
    ThemeProvider = TeletermThemeProvider;
    theme = meta.globals.theme === 'Dark Theme' ? teletermDarkTheme : teletermLightTheme;
  } else {
    ThemeProvider = DefaultThemeProvider;
    theme = meta.globals.theme === 'Dark Theme' ? darkTheme : lightTheme;
  }

  return (
    <ThemeProvider theme={theme}>
      <Box p={3}>{storyFn()}</Box>
    </ThemeProvider>
  );
};

addDecorator(ThemeDecorator);
addParameters({
  options: {
    showPanel: false,
    showNav: true,
    isToolshown: true,
    storySort: {
      method: 'alphabetical',
      order: ['Teleport', 'TeleportE', 'Teleterm', 'Design', 'Shared'],
    },
  },
});

export const globalTypes = {
  theme: {
    name: 'Theme',
    description: 'Global theme for components',
    defaultValue: 'Dark Theme',
    toolbar: {
      icon: 'contrast',
      items: ['Light Theme', 'Dark Theme'],
      dynamicTitle: true,
    },
  },
};
