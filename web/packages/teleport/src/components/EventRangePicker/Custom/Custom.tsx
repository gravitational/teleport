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

import React, { useState, forwardRef } from 'react';
import { isAfter, endOfDay, startOfDay, isSameDay } from 'date-fns';
import { DayPicker, addToRange, DateRange } from 'react-day-picker';
import 'react-day-picker/dist/style.css';

import { StyledDateRange } from 'teleport/components/DayPicker/Shared';

/**
 * Allows user to select any range "from" (no limit)
 * to "to" (no later than "today").
 *
 * @param currentRange is used to initially render the Calendar:
 *   - if fields are undefined, the Calendar will render with
 *     today's month with today's date bolded
 *   - if fields are defined, the Calendar will render with the
 *     provided dates with the range highlighted
 */
export const CustomRange = forwardRef<
  HTMLDivElement,
  {
    currentRange: DateRange;
    onChange(from: Date, to: Date): void;
  }
>(({ currentRange, onChange }, ref) => {
  const [newRange, setNewRange] = useState<DateRange | undefined>();

  function handleDayClick(selectedDay: Date) {
    // Don't let select date past today.
    if (isAfter(selectedDay, endOfDay(new Date()))) {
      return;
    }

    // Don't do anything if `selected day` == `selected from`
    if (newRange?.from && isSameDay(newRange.from, selectedDay)) {
      return;
    }

    let range;
    if (!newRange) {
      // Start
      range = addToRange(selectedDay, { from: undefined, to: undefined });
    } else {
      range = addToRange(selectedDay, { from: newRange.from, to: newRange.to });
    }

    if (range.from) {
      range.from = startOfDay(range.from);
    }

    if (range.to) {
      range.to = endOfDay(range.to);
    }

    setNewRange(range);

    if (range.from && range.to) {
      onChange(range.from, range.to);
    }
  }

  return (
    <StyledDateRange ref={ref}>
      <DayPicker
        mode="range"
        numberOfMonths={2}
        defaultMonth={currentRange.from}
        disabled={{
          after: new Date(),
        }}
        selected={newRange || currentRange}
        onDayClick={handleDayClick}
      />
    </StyledDateRange>
  );
});
