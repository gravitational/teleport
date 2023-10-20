/**
 * Copyright 2023 Gravitational, Inc.
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

import styled from 'styled-components';

interface TooltipPositionProps {
  position: 'left' | 'middle' | 'right';
}

const tooltipPosition = (props: TooltipPositionProps) => {
  if (props.position === 'right') {
    return {
      right: '2px',
    };
  }
  if (props.position === 'middle') {
    return {
      left: '50%',
      transform: 'translateX(-50%)',
    };
  }
  return {
    left: '2px',
  };
};
const tooltipCaretPosition = (props: TooltipPositionProps) => {
  if (props.position === 'right') {
    return {
      right: '10px',
    };
  }
  if (props.position === 'middle') {
    return {
      left: '50%',
      transform: 'translateX(-50%)',
    };
  }
  return {
    left: '10px',
  };
};

export const Tooltip = styled.div<TooltipPositionProps>`
  ${tooltipPosition};

  position: absolute;
  white-space: nowrap;
  pointer-events: none;
  top: 40px;
  z-index: 999;
  background: rgba(0, 0, 0, 0.8);
  color: white;
  border-radius: 7px;
  padding: 5px 8px;

  &:after {
    content: '';
    position: absolute;
    width: 0;
    height: 0;
    border-style: solid;
    border-width: 0 7px 7px 7px;
    border-color: transparent transparent rgba(0, 0, 0, 0.8) transparent;
    top: -7px;

    ${tooltipCaretPosition};
  }
`;
