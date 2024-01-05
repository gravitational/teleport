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

export const CustomRange = forwardRef<
  HTMLDivElement,
  {
    from: Date;
    to: Date;
    onChange(from: Date, to: Date): void;
  }
>(({ from, to, onChange }, ref) => {
  const [range, setRange] = useState<DateRange | undefined>();

  function handleDayClick(selectedDay: Date) {
    if (isAfter(selectedDay, endOfDay(new Date()))) {
      return;
    }

    if (isSameDay(range ? range.from : from, selectedDay)) {
      return;
    }

    let newRange;
    if (!range) {
      // reset the range once a user starts a new selection
      newRange = addToRange(selectedDay, { from: undefined, to: undefined });
    } else {
      newRange = addToRange(selectedDay, { from: range.from, to: range.to });
    }

    if (newRange.from) {
      newRange.from = startOfDay(newRange.from);
    }

    if (newRange.to) {
      newRange.to = endOfDay(newRange.to);
    }

    setRange(newRange);

    if (newRange.from && newRange.to) {
      onChange(newRange.from, newRange.to);
    }
  }

  return (
    <StyledDateRange ref={ref}>
      <DayPicker
        mode="range"
        numberOfMonths={2}
        defaultMonth={from}
        disabled={{
          after: new Date(),
        }}
        selected={range || { from, to }}
        onDayClick={handleDayClick}
      />
    </StyledDateRange>
  );
});
