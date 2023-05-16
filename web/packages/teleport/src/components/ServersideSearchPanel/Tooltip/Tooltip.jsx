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

import React, { createRef } from 'react';
import styled from 'styled-components';
import { Box } from 'design';
import { Info } from 'design/Icon';
import Popover from 'design/Popover';

export default class Tooltip extends React.Component {
  anchorEl = createRef();

  state = {
    open: false,
  };

  onOpen = () => {
    this.setState({ open: true });
  };

  onClose = () => {
    this.setState({ open: false });
  };

  render() {
    const { open } = this.state;
    return (
      <>
        <TooltipButton
          setRef={e => (this.anchorEl = e)}
          onClick={this.onOpen}
          style={{ cursor: 'pointer', fontSize: '20px' }}
        />
        {open && (
          <Popover
            id="tooltip"
            open={open}
            anchorEl={this.anchorEl}
            getContentAnchorEl={null}
            onClose={this.onClose}
            transformOrigin={{
              vertical: 'top',
              horizontal: 'left',
            }}
            anchorOrigin={{
              vertical: 'bottom',
              horizontal: 'center',
            }}
            modalCss={() => 'margin-top: 8px'}
          >
            <PopoverContent p={4}>
              <Box>{this.props.children}</Box>
            </PopoverContent>
          </Popover>
        )}
      </>
    );
  }
}

const PopoverContent = styled(Box)`
  height: fit-content;
  width: fit-content;
  max-width: 536px;
  background: ${props => props.theme.colors.levels.elevated};
`;

const TooltipButton = ({ setRef, ...props }) => {
  return (
    <div ref={setRef} style={{ lineHeight: '0px' }}>
      <Info {...props} />
    </div>
  );
};
