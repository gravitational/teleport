/**
Copyright 2022 Gravitational, Inc.

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

import React, { CSSProperties } from 'react';
import styled from 'styled-components';
import { space, width, color, height } from 'styled-system';

export interface TextAreaProps extends React.ComponentPropsWithRef<'textarea'> {
  hasError?: boolean;
  resizable?: boolean;

  // TS: temporary handles ...styles
  [key: string]: any;
}

export const TextArea: React.FC<TextAreaProps> = styled.textarea`
  appearance: none;
  border: 1px solid ${props => props.theme.colors.text.muted};
  border-radius: 4px;
  box-sizing: border-box;
  min-height: 50px;
  height: 80px;
  font-size: 16px;
  padding: 16px;
  outline: none;
  width: 100%;
  color: ${props => props.theme.colors.text.main};
  background: inherit;

  ::placeholder {
    color: ${props => props.theme.colors.text.muted};
    opacity: 1;
  }

  &:hover,
  &:focus,
  &:active {
    border: 1px solid ${props => props.theme.colors.text.slightlyMuted};
  }

  :read-only {
    cursor: not-allowed;
  }

  :disabled {
    color: ${props => props.theme.colors.text.disabled};
    border-color: ${props => props.theme.colors.text.disabled};
  }

  ${color} ${space} ${width} ${height} ${error} ${resize};
`;

function error({
  hasError,
  theme,
}: Pick<TextAreaProps, 'hasError'> & {
  theme: any;
}) {
  if (!hasError) {
    return;
  }

  return {
    border: `2px solid ${theme.colors.error.main}`,
    '&:hover, &:focus': {
      border: `2px solid ${theme.colors.error.main}`,
    },
  };
}

function resize({
  resizable,
}: Pick<TextAreaProps, 'resizable'>): CSSProperties {
  return { resize: resizable ? 'vertical' : 'none' };
}
