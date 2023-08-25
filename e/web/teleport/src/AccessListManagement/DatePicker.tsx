import React from 'react';
import styled from 'styled-components';
import 'react-day-picker/lib/style.css';
import dayPicker from 'react-day-picker/DayPicker';
import { Flex } from 'design';
import Dialog from 'design/DialogConfirmation';
import { Cross as CloseIcon } from 'design/Icon';

const DayPicker = dayPicker.default || dayPicker;

// TODO(lisa): styles are almost replica of EventRangePicker.tsx
// used for specifying range for audit log.
// Move styles to a more generic place.
export function DatePicker({
  onPickDate,
  selectedDate,
  onClose,
}: {
  onPickDate(date: Date): void;
  selectedDate: Date;
  onClose(): void;
}) {
  function handleOnPickDate(pickedDate: Date) {
    // Prevent user from selecting past dates.
    const today = new Date();
    if (pickedDate < today) {
      return;
    }

    onPickDate(pickedDate);
  }

  return (
    <Dialog
      dialogCss={() => ({ padding: '0' })}
      disableEscapeKeyDown={false}
      onClose={onClose}
      open={true}
    >
      <StyledDateRange>
        <StyledCloseButton title="Close" onClick={onClose}>
          <CloseIcon color="dark" size="medium" />
        </StyledCloseButton>
        <DayPicker
          className="Selectable"
          numberOfMonths={2}
          disabledDays={{
            before: new Date(),
          }}
          onDayClick={date => handleOnPickDate(date)}
          selectedDays={selectedDate}
        />
      </StyledDateRange>
    </Dialog>
  );
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
    height: 300px;
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
