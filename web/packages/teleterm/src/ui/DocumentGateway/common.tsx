/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import FieldInput from 'shared/components/FieldInput';
import styled from 'styled-components';
import React, { forwardRef } from 'react';

export const ConfigFieldInput: typeof FieldInput = styled(FieldInput)`
  input {
    background: inherit;
    border: 1px ${props => props.theme.colors.action.disabledBackground} solid;
    color: ${props => props.theme.colors.text.primary};
    box-shadow: none;
    font-size: 14px;
    height: 34px;

    ::placeholder {
      opacity: 1;
      color: ${props => props.theme.colors.text.secondary};
    }

    &:hover {
      border-color: ${props => props.theme.colors.text.secondary};
    }

    &:focus {
      border-color: ${props => props.theme.colors.brand.main};
    }
  }
`;

const ConfigFieldInputWithoutStepper: typeof ConfigFieldInput = styled(
  ConfigFieldInput
)`
  input {
    ::-webkit-inner-spin-button {
      -webkit-appearance: none;
      margin: 0;
    }

    ::-webkit-outer-spin-button {
      -webkit-appearance: none;
      margin: 0;
    }
  }
`;

export const PortFieldInput: typeof ConfigFieldInput = forwardRef(
  (props, ref) => (
    <ConfigFieldInputWithoutStepper
      type="number"
      min={1}
      max={65535}
      ref={ref}
      {...props}
      width="110px"
    />
  )
);
