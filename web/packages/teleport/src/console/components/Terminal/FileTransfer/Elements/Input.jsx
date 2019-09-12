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
import { width, space } from 'styled-system';
import { colors } from '../../../colors';

const Input = styled.input`
  border: none;
  box-sizing: border-box;
  outline: none;
  width: 360px;
  background-color: ${colors.bgTerminal};
  color: ${colors.terminal};
  ${space}
  ${width}
`

Input.defaultProps = {
  mb: 3,
  mr: 2,
  px: 2,
  py: '4px',
}

export default Input;
