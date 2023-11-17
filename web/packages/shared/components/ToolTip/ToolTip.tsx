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

import React, { useState } from 'react';
import styled from 'styled-components';

import { Text, Popover } from 'design';
import * as Icons from 'design/Icon';

export const ToolTipInfo: React.FC<{
  trigger?: 'click' | 'hover';
  sticky?: boolean;
  maxWidth?: number;
}> = ({ children, trigger = 'hover', sticky = false, maxWidth = 350 }) => {
  const [anchorEl, setAnchorEl] = useState();
  const open = Boolean(anchorEl);

  function handlePopoverOpen(event) {
    setAnchorEl(event.currentTarget);
  }

  function handlePopoverClose() {
    setAnchorEl(null);
  }

  const triggerOnHoverProps = {
    onMouseEnter: handlePopoverOpen,
    onMouseLeave: sticky ? undefined : handlePopoverClose,
  };
  const triggerOnClickProps = {
    onClick: handlePopoverOpen,
  };

  return (
    <>
      <span
        aria-owns={open ? 'mouse-over-popover' : undefined}
        {...(trigger === 'hover' && triggerOnHoverProps)}
        {...(trigger === 'click' && triggerOnClickProps)}
        css={`
          :hover {
            cursor: pointer;
          }
          vertical-align: middle;
          display: inline-block;
          height: 18px;
        `}
      >
        <Icons.Info fontSize={4} />
      </span>
      <Popover
        modalCss={() =>
          trigger === 'hover' && `pointer-events: ${sticky ? 'auto' : 'none'}`
        }
        onClose={handlePopoverClose}
        open={open}
        anchorEl={anchorEl}
        anchorOrigin={{
          vertical: 'bottom',
          horizontal: 'left',
        }}
        transformOrigin={{
          vertical: 'top',
          horizontal: 'left',
        }}
      >
        <StyledOnHover px={3} py={2} $maxWidth={maxWidth}>
          {children}
        </StyledOnHover>
      </Popover>
    </>
  );
};

const StyledOnHover = styled(Text)`
  background-color: white;
  color: black;
  max-width: ${p => p.$maxWidth}px;
`;
