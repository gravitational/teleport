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

import { createGlobalStyle, css } from 'styled-components';

import { getPlatform, Platform } from 'design/platform';

const GlobalStyle = createGlobalStyle`

  html {
    font-family: ${props => props.theme.font};
    ${props => props.theme.typography.body2};
  }

  body {
    margin: 0;
    background-color: ${props => props.theme.colors.levels.sunken};
    color: ${props => props.theme.colors.text.main};
    padding: 0;
  }

  input, textarea {
    font-family: ${props => props.theme.font};
  }

  input {
    accent-color: ${props => props.theme.colors.brand};
  }

  // remove dotted Firefox outline
  button, a {
    outline: 0;

    ::-moz-focus-inner {
      border: 0;
    }
  }

  b, strong {
    // Overrides the default font-weight: bolder which results in the bold font not being bold
    // enough. That's because if the regular font-weight is set to 300, "bolder" means it'll go to
    // just 400.
    // https://developer.mozilla.org/en-US/docs/Web/CSS/font-weight#meaning_of_relative_weights
    //
    // The Web UI uses <Text bold> to render bold text, whereas Connect mostly uses <strong>.
    font-weight: ${props => props.theme.fontWeights.bold};
  }

  ${() => getPlatform() !== Platform.macOS && customScrollbar}
`;

const customScrollbar = css`
  ::-webkit-scrollbar {
    width: 8px;
    height: 8px;
  }

  ::-webkit-scrollbar-thumb {
    background: #757575;
  }

  ::-webkit-scrollbar-corner {
    background: rgba(0, 0, 0, 0.5);
  }
`;

export { GlobalStyle };
