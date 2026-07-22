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

import isPropValid from '@emotion/is-prop-valid';
import { ReactNode } from 'react';
import {
  ThemeProvider as StyledThemeProvider,
  StyleSheetManager,
  WebTarget,
} from 'styled-components';

import { Theme } from 'design/theme';
import { GlobalStyle } from 'design/ThemeProvider/globals';

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

/**
 * Uses a theme from the prop and configures a `styled-components` theme.
 * Can be used in tests, storybook or in an app.
 */
export function ConfiguredThemeProvider(props: {
  theme: Theme;
  children?: ReactNode;
}) {
  return (
    <StyledThemeProvider theme={props.theme}>
      <StyleSheetManager shouldForwardProp={shouldForwardProp}>
        <GlobalStyle />
        {props.children}
      </StyleSheetManager>
    </StyledThemeProvider>
  );
}
