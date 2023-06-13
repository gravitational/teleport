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

import { fonts } from 'design/theme/fonts';
import {
  darkTheme as designDarkTheme,
  lightTheme as designLightTheme,
} from 'design/theme';
import { lighten } from 'design/theme/utils/colorManipulator';

const sansSerif = 'system-ui';

export const darkTheme = {
  ...designDarkTheme,
  colors: {
    ...designDarkTheme.colors,
    terminal: {
      ...designDarkTheme.colors.terminal,
      background: designDarkTheme.colors.levels.sunken,
      cursorAccent: designDarkTheme.colors.levels.sunken,
      brightWhite: lighten(designDarkTheme.colors.levels.sunken, 0.89),
      white: lighten(designDarkTheme.colors.levels.sunken, 0.78),
      brightBlack: lighten(designDarkTheme.colors.levels.sunken, 0.61),
    },
  },
  font: sansSerif,
  fonts: {
    sansSerif,
    mono: fonts.mono,
  },
};

export const lightTheme = {
  ...designLightTheme,
  font: sansSerif,
  fonts: {
    sansSerif,
    mono: fonts.mono,
  },
};
