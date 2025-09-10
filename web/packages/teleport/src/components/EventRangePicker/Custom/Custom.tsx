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

import { endOfDay, isAfter, startOfDay, subMonths } from 'date-fns';
import { forwardRef, useState } from 'react';
import { DateRange, DayPicker } from 'react-day-picker';

import 'react-day-picker/dist/style.css';

import { StyledDateRange } from 'design/DatePicker';

/**
 * Allows user to select any range "from" (no limit)
 * to "to" (no later than "today").
 *
 * @param initialRange is used to initially render the Calendar:
 *   - if fields are undefined, the Calendar will render with
 *     today's month with today's date bolded
 *   - if fields are defined, the Calendar will render with the
 *     provided dates with the range highlighted
 */
export const CustomRange = forwardRef<
  HTMLDivElement,
  {
    initialRange: DateRange;
    onChange(from: Date, to: Date): void;
  }
>(({ initialRange, onChange }, ref) => {
  // selectedRange is initially undefined since
  // it represents the "new" range which upon
  // initial render, nothing has been selected yet.
  const [selectedRange, setSelectedRange] = useState<DateRange>({
    from: undefined,
    to: undefined,
  });

  // This handler runs on *every* click, but we only call onChange
  // once both ends of the range are picked.
  const handleDayClick = (
    range: { to: Date; from: Date },
    dateClicked: Date
  ) => {
    // if no range, that means the to and from are the same
    if (!range) {
      // if nothing is selected and nothing exists before, do noting
      if (!selectedRange.from) {
        return;
      }
      const start = startOfDay(selectedRange.from);
      const end = endOfDay(selectedRange.from);
      setSelectedRange({
        from: start,
        to: end,
      });
      onChange(start, end);
      return;
    }

    const { to, from } = range;
    const start = startOfDay(from);
    const end = endOfDay(to);
    // if we have no selection in state, this is a "new" range
    if (!selectedRange.to && !selectedRange.from) {
      setSelectedRange({
        // by default, if a range has already been selected, if the first date clicked is "after" the previous end, it
        // will just assume we want to update the old end. when really, when want _any_ first click to be the new _start_.
        from: isAfter(dateClicked, from) ? end : start,
        to: undefined,
      });
      return;
    }

    setSelectedRange({
      from: start,
      to: end,
    });
    onChange(start, end);
  };

  return (
    <StyledDateRange ref={ref}>
      <DayPicker
        mode="range"
        numberOfMonths={2}
        defaultMonth={
          selectedRange.from ? selectedRange.from : subMonths(new Date(), 1)
        } // since we are going in the past, the defaultMonth should be on the right side, so we dont have a bunch of disabled days
        disabled={{
          after: new Date(),
        }}
        selected={selectedRange.from ? selectedRange : initialRange}
        onSelect={handleDayClick}
      />
    </StyledDateRange>
  );
});
