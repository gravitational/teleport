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

const StyledTextEditor = styled.div`
  overflow: hidden;
  border-radius: 4px;
  flex: 1;
  display: flex;
  position: relative;
  border: none;
  background: ${props => props.theme.colors.bgTerminal };

  .ace-monokai {
    background: ${props => props.theme.colors.bgTerminal };
  }

  .ace-monokai .ace_marker-layer .ace_active-line {
    color: #FFF;
    background: #000;
  }

  .ace-monokai .ace_gutter,
  .ace-monokai .ace_gutter-cell {
    color: rgba(255, 255, 255, .56);
    background: ${props => props.theme.colors.bgTerminal };
  }

  > .ace_editor {
    position: absolute;
    top: 8px;
    right: 0px;
    bottom: 0px;
    left: 0px;
  }
`

export default StyledTextEditor;