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
import typography, { fontSizes, fontWeights } from 'design/theme/typography';
import { sharedStyles } from 'design/theme/sharedStyles';
import { darkTheme } from 'design/theme';

const sansSerif = 'system-ui';

const theme = {
  name: 'dark',
  colors: darkTheme.colors,
  typography,
  font: sansSerif,
  fonts: {
    sansSerif,
    mono: fonts.mono,
  },
  fontWeights,
  fontSizes,
  space: darkTheme.space,
  borders: darkTheme.borders,
  radii: [0, 2, 4, 8, 16, 9999, '100%'],
  regular: fontWeights.regular,
  bold: fontWeights.bold,
  ...sharedStyles,
  // disabled media queries for styled-system
  breakpoints: [],
};

export default theme;
