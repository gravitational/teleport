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

import { forwardRef } from 'react';
import styled from 'styled-components';

import FieldInput, { FieldInputProps } from 'shared/components/FieldInput';

export const ConfigFieldInput = forwardRef<HTMLInputElement, FieldInputProps>(
  (props, ref) => <FieldInput size="small" ref={ref} {...props} />
);

const ConfigFieldInputWithoutStepper = styled(ConfigFieldInput)`
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

export const PortFieldInput = forwardRef<HTMLInputElement, FieldInputProps>(
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
