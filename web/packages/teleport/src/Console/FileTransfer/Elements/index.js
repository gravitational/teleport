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

import React from 'react';
import styled from 'styled-components';
import { Text, Box } from 'design';
import { colors } from '../../colors';
import Button from './Button';
import ButtonClose from './ButtonClose';
import Input from './Input';
import Label from './Label';

export { Button, ButtonClose, Input, Label };

export const Header = ({ children }) => (
  <Text fontSize={0} bold caps mb={3} children={children} />
);

export const Form = styled(Box)`
  font-size: ${props => props.theme.fontSizes[0]}px;
  background-color: ${colors.dark};
  color: ${colors.terminal};
`;
