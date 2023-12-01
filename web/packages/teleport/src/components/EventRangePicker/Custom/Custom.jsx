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
import { isAfter, endOfDay, startOfDay, isSameDay, subMonths } from 'date-fns';
import styled from 'styled-components';
import dayPicker from 'react-day-picker/DayPicker';
import 'react-day-picker/lib/style.css';
import { Flex } from 'design';
import { Cross as CloseIcon } from 'design/Icon';

// There is a vite issue with react-day-picker in production builds
// https://github.com/vitejs/vite/issues/2139
// TODO(ryan): After node v18 upgrade, swap to the Vite config approach instead of this one
// also, we should look into upgrading react-day-picker
const DayPicker = dayPicker.default || dayPicker;

export default class CustomRange extends React.Component {
  constructor(props) {
    super();
    this.startSelecting = false;
    const { from, to } = props;
    this.state = {
      from,
      to,
    };
  }

  handleDayClick = day => {
    if (isAfter(day, endOfDay(new Date()))) {
      return;
    }

    let { from, end } = this.state;

    if (isSameDay(from, day)) {
      return;
    }

    if (this.startSelecting === false) {
      // reset the range once a user starts a new selection
      end = undefined;
      from = undefined;
      this.startSelecting = true;
    }

    const range = dayPicker.DateUtils.addDayToRange(day, { from, end });

    if (range.from) {
      range.from = startOfDay(range.from);
    }

    if (range.to) {
      range.to = endOfDay(range.to);
    }

    this.setState(range, this.onChange);
  };

  onChange() {
    const { from, to } = this.state;
    if (from && to && this.startSelecting) {
      this.props.onChange(from, to);
    }
  }

  render() {
    const { from, to } = this.state;
    const modifiers = { start: from, end: to };

    return (
      <StyledDateRange>
        <StyledCloseButton title="Close" onClick={this.props.onClosePicker}>
          <CloseIcon color="dark" size="medium" />
        </StyledCloseButton>
        <DayPicker
          className="Selectable"
          numberOfMonths={2}
          month={subMonths(to || new Date(), 1)}
          disabledDays={{
            after: new Date(),
          }}
          selectedDays={[from, { from, to }]}
          modifiers={modifiers}
          onDayClick={this.handleDayClick}
        />
      </StyledDateRange>
    );
  }
}

const StyledCloseButton = styled.button`
  background: transparent;
  border-radius: 2px;
  border: none;
  color: ${props => props.theme.colors.grey[900]};
  cursor: pointer;
  height: 24px;
  width: 24px;
  outline: none;
  padding: 0;
  margin: 0 8px 0 0;
  transition: all 0.3s;
  position: absolute;
  font-size: 20px;
  z-index: 100;
  top: 8px;
  right: 0px;

  display: flex;
  align-items: center;
  justify-content: center;

  &:hover {
    background: ${props => props.theme.colors.grey[200]};
  }
`;

const StyledDateRange = styled(Flex)`
  position: relative;

  .DayPicker {
    line-height: initial;
    color: black;
    background-color: white;
    box-shadow: inset 0 2px 4px rgba(0, 0, 0, 0.24);
    box-sizing: border-box;
    border-radius: 5px;
    padding: 24px;
  }

  .DayPicker-Months {
  }

  .DayPicker-Day--selected:not(.DayPicker-Day--start):not(.DayPicker-Day--end):not(.DayPicker-Day--outside) {
    background-color: #f0f8ff !important;
    color: #4a90e2;
  }

  .DayPicker-Day {
    border-radius: 0 !important;
  }

  .DayPicker-Day--start {
    border-top-left-radius: 50% !important;
    border-bottom-left-radius: 50% !important;
  }

  .DayPicker-Day--end {
    border-top-right-radius: 50% !important;
    border-bottom-right-radius: 50% !important;
  }
`;
