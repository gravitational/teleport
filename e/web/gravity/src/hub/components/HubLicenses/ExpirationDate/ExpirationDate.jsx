import React from 'react';
import styled from 'styled-components';
import DayPickerInput from 'react-day-picker/DayPickerInput';
import MomentLocaleUtils, { formatDate, parseDate } from 'react-day-picker/moment';
import 'react-day-picker/lib/style.css';
import cfg from 'gravity/config';
import { Input } from 'design';

export default class ExpirationDate extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      selectedDay: undefined,
      isEmpty: true,
      isDisabled: false,
    };
  }

  handleDayChange = (selectedDay, modifiers, dayPickerInput) => {
    this.props.onChange(selectedDay);
    const input = dayPickerInput.getInput();
    this.setState({
      selectedDay,
      isEmpty: !input.value.trim(),
      isDisabled: modifiers.disabled === true,
    });
  }

  render() {
    const { selectedDay } = this.state;
    return (
      <StyledDateRange>
        <DayPickerInput
          formatDate={formatDate}
          parseDate={parseDate}
          component={Input}
          placeholder={cfg.dateFormat}
          format={cfg.dateFormat}
          value={selectedDay}
          onDayChange={this.handleDayChange}
          inputProps={{
            autoComplete: "off",
            mb: "0"
          }}
          dayPickerProps={{
            localeUtils: MomentLocaleUtils,
            disabledDays: {
              before: new Date(),
            }
          }}
        />
      </StyledDateRange>
    );
  }
}

const StyledDateRange = styled.div`
  .DayPickerInput-Overlay{
    background-color: transparent;
  }

  .DayPickerInput{
    width: 100%;
  }

  .DayPicker {
    line-height: initial;
    color: black;
    background-color: white;
    box-shadow: inset 0 2px 4px rgba(0,0,0,.24);
    box-sizing: border-box;
    border-radius: 5px;
    padding: 14px;
  }

  .DayPicker-Day {
    border-radius: 0 !important;
  }
`