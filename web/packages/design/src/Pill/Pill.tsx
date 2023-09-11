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

import React from 'react';
import styled from 'styled-components';

import { Cross } from '../Icon';

function Pill({ label, onDismiss }: Props) {
  const dismissable = !!onDismiss;
  return (
    <Wrapper dismissable={dismissable}>
      <Label>{label}</Label>
      <Dismiss
        role="button"
        dismissable={dismissable}
        onClick={(e: MouseEvent) => {
          e.stopPropagation();
          onDismiss(label);
        }}
      >
        <Cross />
      </Dismiss>
    </Wrapper>
  );
}

const Wrapper = styled.span`
  background: ${props => props.theme.colors.spotBackground[1]};
  border-radius: 35px;
  cursor: default;
  display: inline-block;
  padding: ${props => (props.dismissable ? '6px 6px 6px 14px;' : '6px 14px;')};
  white-space: nowrap;
`;

const Label = styled.span`
  display: inline;
`;

const Dismiss = styled.button`
  border-color: rgba(0, 0, 0, 0);
  background-color: rgba(0, 0, 0, 0);
  cursor: pointer;
  display: ${props => (props.dismissable ? 'inline-block' : 'none')};
`;

type Props = {
  label: string;
  dismissable?: boolean;
  onDismiss?: (labelName: string) => void;
};

export { Pill };
