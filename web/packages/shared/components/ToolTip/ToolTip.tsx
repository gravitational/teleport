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

import { Popover, Text } from 'design';
import * as Icons from 'design/Icon';

export const ToolTipInfo: React.FC<{ muteIconColor?: boolean }> = ({
  children,
  muteIconColor,
}) => {
  const [anchorEl, setAnchorEl] = useState();
  const open = Boolean(anchorEl);

  function handlePopoverOpen(event) {
    setAnchorEl(event.currentTarget);
  }

  function handlePopoverClose() {
    setAnchorEl(null);
  }

  return (
    <>
      <span
        role="icon"
        aria-owns={open ? 'mouse-over-popover' : undefined}
        onMouseEnter={handlePopoverOpen}
        onMouseLeave={handlePopoverClose}
        css={`
          :hover {
            cursor: pointer;
          }
          vertical-align: middle;
          display: inline-block;
          height: 18px;
        `}
      >
        <InfoIcon muteIconColor={muteIconColor} />
      </span>
      <Popover
        modalCss={modalCss}
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
        <StyledOnHover px={3} py={2}>
          {children}
        </StyledOnHover>
      </Popover>
    </>
  );
};

const modalCss = () => `
  pointer-events: none;
`;

const StyledOnHover = styled(Text)`
  color: ${props => props.theme.colors.text.main};
  background-color: ${props => props.theme.colors.tooltip.background};
  max-width: 350px;
`;

const InfoIcon = styled(Icons.Info)`
  font-size: 18px;
  color: ${p => (p.muteIconColor ? p.theme.colors.text.disabled : 'inherit')};
`;
