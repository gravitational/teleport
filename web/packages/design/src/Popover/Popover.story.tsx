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

import { Component, MouseEvent, useState } from 'react';
import styled from 'styled-components';

import Popover, { GrowDirections, Origin } from '.';
import { Box, ButtonPrimary, Flex, H2, Text } from '..';

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

type SimplePopoverState = {
  anchorEl: Element | null;
  anchorOrigin?: Origin;
  transformOrigin?: Origin;
  growDirections?: GrowDirections;
  contentMultiplier: number;
};

class SimplePopover extends Component<any, SimplePopoverState> {
  btnRef: Element | null = null;
  growContentTimer: ReturnType<typeof setInterval> | undefined;

  state: SimplePopoverState = {
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

class MouseOverPopover extends Component {
  state = {
    anchorEl: null,
  };

  handlePopoverOpen = (event: MouseEvent<HTMLDivElement>) => {
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

type PopoverSet = {
  anchorOrigin: Origin;
  transformOrigin: Origin;
  name: string;
}[];

const popoverSets: PopoverSet[] = [
  [
    {
      anchorOrigin: { horizontal: 'center', vertical: 'top' },
      transformOrigin: { horizontal: 'center', vertical: 'bottom' },
      name: 'Top',
    },
    {
      anchorOrigin: { horizontal: 'right', vertical: 'center' },
      transformOrigin: { horizontal: 'left', vertical: 'center' },
      name: 'Right',
    },
    {
      anchorOrigin: { horizontal: 'center', vertical: 'bottom' },
      transformOrigin: { horizontal: 'center', vertical: 'top' },
      name: 'Bottom',
    },
    {
      anchorOrigin: { horizontal: 'left', vertical: 'center' },
      transformOrigin: { horizontal: 'right', vertical: 'center' },
      name: 'Left',
    },
  ],
  [
    {
      anchorOrigin: { horizontal: 'left', vertical: 'top' },
      transformOrigin: { horizontal: 'left', vertical: 'bottom' },
      name: 'Top',
    },
    {
      anchorOrigin: { horizontal: 'right', vertical: 'top' },
      transformOrigin: { horizontal: 'left', vertical: 'top' },
      name: 'Right',
    },
    {
      anchorOrigin: { horizontal: 'right', vertical: 'bottom' },
      transformOrigin: { horizontal: 'right', vertical: 'top' },
      name: 'Bottom',
    },
    {
      anchorOrigin: { horizontal: 'left', vertical: 'bottom' },
      transformOrigin: { horizontal: 'right', vertical: 'bottom' },
      name: 'Left',
    },
  ],
  [
    {
      anchorOrigin: { horizontal: 'right', vertical: 'top' },
      transformOrigin: { horizontal: 'right', vertical: 'bottom' },
      name: 'Top',
    },
    {
      anchorOrigin: { horizontal: 'right', vertical: 'bottom' },
      transformOrigin: { horizontal: 'left', vertical: 'bottom' },
      name: 'Right',
    },
    {
      anchorOrigin: { horizontal: 'left', vertical: 'bottom' },
      transformOrigin: { horizontal: 'left', vertical: 'top' },
      name: 'Bottom',
    },
    {
      anchorOrigin: { horizontal: 'left', vertical: 'top' },
      transformOrigin: { horizontal: 'right', vertical: 'top' },
      name: 'Left',
    },
  ],
  [
    {
      anchorOrigin: { horizontal: 20, vertical: 'top' },
      transformOrigin: { horizontal: 30, vertical: 'bottom' },
      name: 'Top',
    },
    {
      anchorOrigin: { horizontal: 'right', vertical: 40 },
      transformOrigin: { horizontal: 'left', vertical: 25 },
      name: 'Right',
    },
    {
      anchorOrigin: { horizontal: 80, vertical: 'bottom' },
      transformOrigin: { horizontal: 60, vertical: 'top' },
      name: 'Bottom',
    },
    {
      anchorOrigin: { horizontal: 'left', vertical: 60 },
      transformOrigin: { horizontal: 'right', vertical: 45 },
      name: 'Left',
    },
  ],
];

export const Positioning = () => {
  return (
    <>
      <H2 my={2}>Without arrows</H2>
      <Flex flexWrap="wrap">
        {popoverSets.map((popovers, i) => (
          <ManyPopovers popovers={popovers} key={i} />
        ))}
      </Flex>
      <H2 my={2}>With arrows</H2>
      <Flex flexWrap="wrap">
        {popoverSets.map((popovers, i) => (
          <ManyPopovers popovers={popovers} key={i} arrows />
        ))}
        <ManyPopovers
          arrows
          anchorSize="200px"
          margin="30px"
          popovers={[
            {
              anchorOrigin: { horizontal: 'center', vertical: 'top' },
              transformOrigin: { horizontal: 'center', vertical: 'top' },
              name: 'Top',
            },
            {
              anchorOrigin: { horizontal: 'right', vertical: 'center' },
              transformOrigin: { horizontal: 'right', vertical: 'center' },
              name: 'Right',
            },
            {
              anchorOrigin: { horizontal: 'center', vertical: 'bottom' },
              transformOrigin: { horizontal: 'center', vertical: 'bottom' },
              name: 'Bottom',
            },
            {
              anchorOrigin: { horizontal: 'left', vertical: 'center' },
              transformOrigin: { horizontal: 'left', vertical: 'center' },
              name: 'Left',
            },
          ]}
        />
      </Flex>
      <H2 my={2}>With arrows and margins</H2>
      <Flex flexWrap="wrap">
        {popoverSets.map((popovers, i) => (
          <ManyPopovers popovers={popovers} key={i} arrows popoverMargin={5} />
        ))}
      </Flex>
    </>
  );
};

const ManyPopovers = ({
  popovers,
  arrows,
  anchorSize = '100px',
  margin = '80px',
  popoverMargin = 0,
}: {
  popovers: PopoverSet;
  arrows?: boolean;
  anchorSize?: string;
  margin?: string;
  popoverMargin?: number;
}) => {
  const [anchorRef, setAnchorRef] = useState<HTMLDivElement | null>(null);
  return (
    <Box backgroundColor="levels.deep">
      <Box
        ref={setAnchorRef}
        border={2}
        width={anchorSize}
        height={anchorSize}
        margin={margin}
      />
      {popovers.map(({ anchorOrigin, transformOrigin, name }) => (
        <Popover
          key={name}
          anchorEl={anchorRef}
          open={!!anchorRef}
          anchorOrigin={anchorOrigin}
          transformOrigin={transformOrigin}
          arrow={arrows}
          popoverMargin={popoverMargin}
        >
          <Box padding={3}>{name}</Box>
        </Popover>
      ))}
    </Box>
  );
};
