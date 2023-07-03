/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { forwardRef } from 'react';
import styled from 'styled-components';

import FieldInput from 'shared/components/FieldInput';
import { requiredField } from 'shared/components/Validation/rules';
import Validation from 'shared/components/Validation';

export const Form = styled.form.attrs(() => ({
  'aria-label': 'form',
}))``;

export const PathInput = forwardRef<
  HTMLInputElement,
  React.ComponentProps<typeof FieldInput>
>((props, ref) => {
  function moveCaretAtEnd(e: React.ChangeEvent<HTMLInputElement>): void {
    const tmp = e.target.value;
    e.target.value = '';
    e.target.value = tmp;
  }

  return (
    <Validation>
      {({ validator }) => (
        <StyledFieldInput
          {...props}
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
});

const StyledFieldInput = styled(FieldInput)`
  input {
    font-size: 14px;
    height: 32px;
  }
`;
