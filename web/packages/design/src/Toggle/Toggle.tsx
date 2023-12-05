/*
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

import React, { ReactNode } from 'react';
import styled from 'styled-components';

export function Toggle({
  isToggled,
  onToggle,
  children,
  disabled,
  className,
}: {
  isToggled: boolean;
  onToggle: () => void;
  children?: ReactNode;
  disabled?: boolean;
  className?: string;
}) {
  return (
    <StyledWrapper disabled={disabled} className={className}>
      <StyledInput
        checked={isToggled}
        onChange={onToggle}
        disabled={disabled}
      />
      <StyledSlider />
      {children}
    </StyledWrapper>
  );
}

const StyledWrapper = styled.label<{ disabled: boolean }>`
  position: relative;
  display: flex;
  align-items: center;
  cursor: ${p => p.disabled ? 'default' : 'pointer'};
`;

const StyledSlider = styled.div`
  width: 32px;
  height: 12px;
  border-radius: 12px;
  background: ${props => props.theme.colors.levels.surface};
  cursor: inherit;
  flex-shrink: 0;

  &:before {
    content: '';
    position: absolute;
    top: 50%;
    transform: translate(0, -50%);
    width: 16px;
    height: 16px;
    border-radius: 16px;
    background: ${props => props.theme.colors.brand};
  }
`;

const StyledInput = styled.input.attrs({ type: 'checkbox' })`
  opacity: 0;
  position: absolute;
  cursor: inherit;

  &:checked + ${StyledSlider} {
    background: ${props => props.theme.colors.spotBackground[1]};

    &:before {
      transform: translate(16px, -50%);
    }
  }

  &:disabled + ${StyledSlider} {
    background: ${props => props.theme.colors.levels.surface};

    &:before {
      background: ${props => props.theme.colors.grey[700]};
    }
  }
`;
