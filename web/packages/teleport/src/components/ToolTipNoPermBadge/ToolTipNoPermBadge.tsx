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
import { Text, Popover, Box } from 'design';

type Props = {
  borderRadius?: number;
};

export const ToolTipNoPermBadge: React.FC<Props> = ({
  children,
  borderRadius = 2,
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
      <Box
        data-testid="tooltip"
        aria-owns={open ? 'mouse-over-popover' : undefined}
        onMouseEnter={handlePopoverOpen}
        onMouseLeave={handlePopoverClose}
        borderTopRightRadius={borderRadius}
        borderBottomLeftRadius={borderRadius}
        css={`
          position: absolute;
          padding: 0px 6px;
          top: 0px;
          right: 0px;
          font-size: 10px;
          background-color: ${p => p.theme.colors.error.main};
        `}
      >
        Lacking Permissions
      </Box>
      <Popover
        modalCss={() => `pointer-events: none;`}
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
        <StyledOnHover px={3} py={2} data-testid="tooltip-msg">
          {children}
        </StyledOnHover>
      </Popover>
    </>
  );
};

const StyledOnHover = styled(Text)`
  background-color: white;
  color: black;
  max-width: 350px;
`;
