import { forwardRef } from 'react';
import 'react-day-picker/style.css';
import { DayPicker } from 'react-day-picker';

import { StyledDateRange } from 'design/DatePicker';
import Dialog from 'design/DialogConfirmation';

// TODO(lisa): instead of a dialog change into a dropdown
// so we can avoid dialog on dialog:
// https://github.com/gravitational/teleport.e/pull/3095#discussion_r1442961504
export const DatePicker = forwardRef<
  HTMLDivElement,
  {
    onPickDate(date: Date): void;
    selectedDate: Date;
  }
>(({ onPickDate, selectedDate }, ref) => {
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
      open={true}
    >
      <StyledDateRange ref={ref}>
        <DayPicker
          numberOfMonths={2}
          disabled={{
            before: new Date(),
          }}
          onDayClick={date => handleOnPickDate(date)}
          selected={selectedDate}
        />
      </StyledDateRange>
    </Dialog>
  );
});
