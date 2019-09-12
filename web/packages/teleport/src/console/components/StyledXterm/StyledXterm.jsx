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

import styled from 'styled-components';
import { Box } from 'design';
import 'xterm/dist/xterm.css';
import { colors } from './../colors';

const StyledXterm = styled(Box)`
  height: 100%;
  width: 100%;
  font-size: 14px;
  line-height: normal;
  overflow: auto;
  background-color: ${colors.bgTerminal};

  .terminal {
    font-family: ${props => props.theme.fonts.mono};
    border: none;
    font-size: inherit;
    line-height: normal;
    position: relative;
  }

  .terminal .xterm-viewport {
    background-color:${colors.bgTerminal};
    overflow-y: hidden;
  }

  .terminal * {
    font-weight: normal!important;
  }
`;

export default StyledXterm;