/*
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

import React from 'react';
import { configure } from '@storybook/react';
import { addDecorator } from '@storybook/react';
import theme from './../../theme';
import ThemeProvider from '../ThemeProvider';
import Box from './../Box';

const ThemeDecorator = storyFn => (
  <ThemeProvider theme={theme}>
    <Box p={3}>{storyFn()}</Box>
  </ThemeProvider>
);

addDecorator(ThemeDecorator);

const sharedReq = require.context('./../../', true, /\.story.js$/);
const srcReq = require.context('./../../../src', true, /\.story.js$/);

const loadStories = () => {
  sharedReq.keys().forEach(sharedReq);
  srcReq.keys().forEach(srcReq);
};

configure(loadStories, module);
