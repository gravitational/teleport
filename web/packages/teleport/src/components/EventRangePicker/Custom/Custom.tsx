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

import { endOfDay, isAfter, isSameDay, startOfDay } from 'date-fns';
import { forwardRef, useState } from 'react';
import { addToRange, DateRange, DayPicker } from 'react-day-picker';

import 'react-day-picker/dist/style.css';

import { StyledDateRange } from 'teleport/components/DayPicker/Shared';

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

  function handleDayClick(selectedDay: Date) {
    // Don't let select date past today.
    if (isAfter(selectedDay, endOfDay(new Date()))) {
      return;
    }

    // Don't do anything if `selected day` == `selected from`
    if (selectedRange?.from && isSameDay(selectedRange.from, selectedDay)) {
      return;
    }

    const newRange = addToRange(selectedDay, selectedRange);

    if (newRange.from) {
      newRange.from = startOfDay(newRange.from);
    }

    if (newRange.to) {
      newRange.to = endOfDay(newRange.to);
    }

    setSelectedRange(newRange);

    if (newRange.from && newRange.to) {
      onChange(newRange.from, newRange.to);
    }
  }

  return (
    <StyledDateRange ref={ref}>
      <DayPicker
        mode="range"
        numberOfMonths={2}
        defaultMonth={initialRange.from}
        disabled={{
          after: new Date(),
        }}
        selected={selectedRange.from ? selectedRange : initialRange}
        onDayClick={handleDayClick}
      />
    </StyledDateRange>
  );
});
