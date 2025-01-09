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

import { createGlobalStyle } from 'styled-components';

import './../assets/ubuntu/style.css';

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

    &::placeholder {
      color: ${props => props.theme.colors.text.muted};
    }
  }

  // custom scrollbars with the ability to use the default scrollbar behavior via adding the attribute [data-scrollbar=default]
  :not([data-scrollbar="default"])::-webkit-scrollbar {
    width: 8px;
    height: 8px;
  }

  :not([data-scrollbar="default"])::-webkit-scrollbar-thumb {
    background: #757575;
  }

  :not([data-scrollbar="default"])::-webkit-scrollbar-corner {
    background: rgba(0,0,0,0.5);
  }

  :root {
    color-scheme: ${props =>
      props.theme
        .name}; // this ensures Chrome's scrollbars are set to the right color depending on the theme
  }

  // remove dotted Firefox outline
  button, a {
    outline: 0;
    ::-moz-focus-inner {
      border: 0;
    }
  }
`;

export { GlobalStyle };
