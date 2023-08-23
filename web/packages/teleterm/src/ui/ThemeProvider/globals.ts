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

import { createGlobalStyle, css } from 'styled-components';
import { getPlatform } from 'design/theme/utils';

const GlobalStyle = createGlobalStyle`

  html {
    font-family: ${props => props.theme.font};
    ${props => props.theme.typography.body1};
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

  // remove dotted Firefox outline
  button, a {
    outline: 0;

    ::-moz-focus-inner {
      border: 0;
    }
  }

  ${() => !getPlatform().isMac && customScrollbar}
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
