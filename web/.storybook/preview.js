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
import { addDecorator, addParameters } from '@storybook/react';
import theme from './../packages/design/src/theme';
import ThemeProvider from './../packages/design/src/ThemeProvider';
import Box from './../packages/design/src/Box';

// wrap each story with gravitational theme provider
const ThemeDecorator = storyFn => (
  <ThemeProvider theme={theme}>
    <Box p={3}>{storyFn()}</Box>
  </ThemeProvider>
);

// alphabetically sort stories
function storySort(a, b) {
  return a[1].kind === b[1].kind
    ? 0
    : a[1].id.localeCompare(b[1].id, undefined, { numeric: true });
}

addDecorator(ThemeDecorator);
addParameters({
  options: {
    showPanel: false,
    showNav: true,
    isToolshown: true,
    storySort,
  },
});
