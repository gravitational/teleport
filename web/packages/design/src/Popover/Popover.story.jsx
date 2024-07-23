/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import React from 'react';
import styled from 'styled-components';

import { ButtonPrimary, Box, Flex, Text } from '..';

import Popover from '.';

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
    contentMultiplier: 1,
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

  bottomRightGrowDirection = () => {
    this.setState({
      anchorEl: this.btnRef,
      growDirections: 'bottom-right',
    });
    this.startGrowContent();
  };

  topLeftGrowDirection = () => {
    this.setState({
      anchorEl: this.btnRef,
      growDirections: 'top-left',
    });
    this.startGrowContent();
  };

  startGrowContent = () => {
    this.growContentTimer = setInterval(() => {
      if (this.state.contentMultiplier > 20) {
        clearInterval(this.growContentTimer);
        return;
      }
      this.setState(prevState => ({
        anchorEl: this.btnRef,
        contentMultiplier: prevState.contentMultiplier + 1,
      }));
    }, 500);
  };

  handleClose = () => {
    this.setState({
      anchorEl: null,
      contentMultiplier: 1,
    });
    clearInterval(this.growContentTimer);
  };

  render() {
    const { anchorEl, anchorOrigin, transformOrigin, growDirections } =
      this.state;
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
            growDirections={growDirections}
            anchorEl={anchorEl}
            onClose={this.handleClose}
          >
            <Box bg="white" color="black" p={5} data-testid="content">
              {'The content of the Popover. '.repeat(
                this.state.contentMultiplier
              )}
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
        <Flex m={11} justifyContent="space-around">
          <ButtonPrimary size="small" onClick={this.bottomRightGrowDirection}>
            Bottom - Right grow direction
          </ButtonPrimary>
          <ButtonPrimary size="small" onClick={this.topLeftGrowDirection}>
            Top - Left grow direction
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
            data-testid="text"
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
          <StyledOnHover p={1} data-testid="content">
            Sample popover text. (tooltip)
          </StyledOnHover>
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
