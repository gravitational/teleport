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
import { configure, addParameters } from '@storybook/react';
import { addDecorator } from '@storybook/react';
import theme from './../packages/design/src/theme';
import ThemeProvider from './../packages/design/src/ThemeProvider';
import Box from './../packages/design/src/Box';

const reqs = require.context('./../packages/', true, /\.story.(js|tsx)$/);

function storySort(moduleA, moduleB) {
  return moduleA[1].id.localeCompare(moduleB[1].id);
}

function loadStories() {
  reqs.keys().forEach(reqs);
}

const ThemeDecorator = storyFn => (
  <ThemeProvider theme={theme}>
    <Box p={3}>{storyFn()}</Box>
  </ThemeProvider>
);

addDecorator(ThemeDecorator);
addParameters({
  options: {
    showPanel: false,
    showNav: true,
    isToolshown: true,
    storySort,
  },
});

configure(loadStories, module);
