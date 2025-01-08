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

import React, { forwardRef } from 'react';
import styled from 'styled-components';

import FieldInput, { FieldInputProps } from 'shared/components/FieldInput';
import Validation from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

export const Form = styled.form.attrs(() => ({
  'aria-label': 'form',
}))``;

export const PathInput = forwardRef<HTMLInputElement, FieldInputProps>(
  (props, ref) => {
    function moveCaretAtEnd(e: React.ChangeEvent<HTMLInputElement>): void {
      const tmp = e.target.value;
      e.target.value = '';
      e.target.value = tmp;
    }

    return (
      <Validation>
        {({ validator }) => (
          <FieldInput
            {...props}
            size="small"
            onFocus={moveCaretAtEnd}
            ref={ref}
            spellCheck={false}
            mb={0}
            mt={0}
            width="100%"
            onBlur={() => validator.validate()}
            rule={requiredField('Path is required')}
          />
        )}
      </Validation>
    );
  }
);
