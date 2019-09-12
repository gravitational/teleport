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

import React from 'react'
import moment from 'moment';
import styled from 'styled-components';
import DayPicker, { DateUtils } from 'react-day-picker';
import 'react-day-picker/lib/style.css';
import { Flex } from 'design';

export default class CustomRange extends React.Component {

  constructor(props){
    super();
    this.startSelecting = false;
    const { from, to } = props;
    this.state = {
      from,
      to,
    }
  }

  handleDayClick = day => {
    if(moment(day) > moment(new Date()).endOf('day')){
      return;
    }

    let { from, end } = this.state;

    if(moment(from).isSame(day)){
      return;
    }

    if(this.startSelecting === false){
      // reset the range once a user starts a new selection
      end = undefined;
      from = undefined;
      this.startSelecting = true;
    }

    const range = DateUtils.addDayToRange(day, { from, end });
    this.setState(range, this.onChange);
  }

  onChange(){
    const { from, to } = this.state;
    if(from && to && this.startSelecting){
      // make sure that range includes the end/start of the day
      const start = moment(from).startOf('day').toDate();
      const end = moment(to).endOf('day').toDate();
      this.props.onChange(start, end);
    }
  }

  render() {
    const { from, to } = this.state;
    const modifiers = { start: from, end: to };

    return (
      <StyledDateRange>
        <DayPicker
          className="Selectable"
          numberOfMonths={2}
          month={moment(to).subtract(1, 'month').toDate()}
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

const StyledDateRange = styled(Flex)`
  position: relative;

  .DayPicker {
    line-height: initial;
    color: black;
    background-color: white;
    box-shadow: inset 0 2px 4px rgba(0,0,0,.24);
    box-sizing: border-box;
    border-radius: 5px;
    padding: 24px;
  }

  .DayPicker-Months{
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
`