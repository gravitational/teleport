/*
Copyright 2019 Gravitational, Inc.

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

import React from 'react';
import styled from 'styled-components';
import Popover from './../Popover';
import { ButtonPrimary, Box, Flex, Text } from '../';

export default {
  title: 'Design/Popover',
};

export const Sample = () => (
  <Box m={11} textAlign="center">
    <SimplePopover />
  </Box>
);

export const Tooltip = () => (
  <Box m={11} textAlign="center">
    <MouseOverPopover />
  </Box>
);

class SimplePopover extends React.Component {
  state = {
    anchorEl: null,
  };

  handleClick = () => {
    this.setState({
      anchorEl: this.btnRef,
    });
  };

  topCenter = () => {
    this.setState({
      anchorEl: this.btnRef,
      anchorOrigin: {
        vertical: 'top',
        horizontal: 'center',
      },
      transformOrigin: {
        vertical: 'bottom',
        horizontal: 'center',
      },
    });
  };

  centerCenter = () => {
    this.setState({
      anchorEl: this.btnRef,
      anchorOrigin: {
        vertical: 'center',
        horizontal: 'center',
      },
      transformOrigin: {
        vertical: 'center',
        horizontal: 'center',
      },
    });
  };

  left = () => {
    this.setState({
      anchorEl: this.btnRef,
      anchorOrigin: {
        vertical: 'bottom',
        horizontal: 'right',
      },
      transformOrigin: {
        vertical: 'top',
        horizontal: 'right',
      },
    });
  };

  right = () => {
    this.setState({
      anchorEl: this.btnRef,
      anchorOrigin: {
        vertical: 'bottom',
        horizontal: 'left',
      },
      transformOrigin: {
        vertical: 'top',
        horizontal: 'left',
      },
    });
  };

  bottomCenter = () => {
    this.setState({
      anchorEl: this.btnRef,
      anchorOrigin: {
        vertical: 'bottom',
        horizontal: 'center',
      },
      transformOrigin: {
        vertical: 'top',
        horizontal: 'center',
      },
    });
  };

  handleClose = () => {
    this.setState({
      anchorEl: null,
    });
  };

  render() {
    const { anchorEl, anchorOrigin, transformOrigin } = this.state;
    const open = Boolean(anchorEl);

    return (
      <div>
        <Box>
          <ButtonPrimary mt={7} setRef={e => (this.btnRef = e)}>
            This is anchor element
          </ButtonPrimary>
          <Popover
            id="simple-popper"
            open={open}
            anchorOrigin={anchorOrigin}
            transformOrigin={transformOrigin}
            anchorEl={anchorEl}
            onClose={this.handleClose}
          >
            <Box bg="white" color="black" p={5}>
              The content of the Popover.
            </Box>
          </Popover>
        </Box>
        <Flex m={11} justifyContent="space-around">
          <ButtonPrimary size="small" onClick={this.left}>
            Left
          </ButtonPrimary>
          <ButtonPrimary size="small" onClick={this.right}>
            Right
          </ButtonPrimary>
          <ButtonPrimary size="small" onClick={this.centerCenter}>
            Center Center
          </ButtonPrimary>
          <ButtonPrimary size="small" onClick={this.topCenter}>
            Top Center
          </ButtonPrimary>
          <ButtonPrimary size="small" onClick={this.bottomCenter}>
            Bottom Center
          </ButtonPrimary>
        </Flex>
      </div>
    );
  }
}

class MouseOverPopover extends React.Component {
  state = {
    anchorEl: null,
  };

  handlePopoverOpen = event => {
    this.setState({ anchorEl: event.currentTarget });
  };

  handlePopoverClose = () => {
    this.setState({ anchorEl: null });
  };

  render() {
    const { anchorEl } = this.state;
    const open = Boolean(anchorEl);
    return (
      <Flex justifyContent="center">
        <Box style={{ height: '200', width: '200px' }}>
          <Text
            aria-owns={open ? 'mouse-over-popover' : undefined}
            onMouseEnter={this.handlePopoverOpen}
            onMouseLeave={this.handlePopoverClose}
          >
            Hover with a Popover.
          </Text>
        </Box>
        <Popover
          modalCss={modalCss}
          onClose={this.handlePopoverClose}
          open={open}
          anchorEl={anchorEl}
          anchorOrigin={{
            vertical: 'bottom',
            horizontal: 'center',
          }}
          transformOrigin={{
            vertical: 'top',
            horizontal: 'center',
          }}
        >
          <StyledOnHover p={1}>Sample popover text. (tooltip)</StyledOnHover>
        </Popover>
      </Flex>
    );
  }
}

const modalCss = () => `
  pointer-events: none;
`;

const StyledOnHover = styled(Text)`
  background-color: white;
  color: black;
`;
