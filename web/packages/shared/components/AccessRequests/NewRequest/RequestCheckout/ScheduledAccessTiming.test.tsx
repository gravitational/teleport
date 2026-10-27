/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { displayTimeWithTimeZone } from 'design/datetime';
import { fireEvent, render, screen } from 'design/utils/testing';
import Validation from 'shared/components/Validation';

import { getFormattedDurationTxt } from '../../Shared/utils';
import { ScheduledAccessTiming } from './ScheduledAccessTiming';

afterEach(() => {
  jest.useRealTimers();
});

test('future scheduled layout renders editable start and end controls', () => {
  // Setup a future scheduled window and its end-change callback.
  jest.useFakeTimers().setSystemTime(new Date('2026-10-19T20:00:00Z'));
  const start = new Date('2026-10-20T16:00:00Z');
  const end = new Date('2026-10-20T18:00:00Z');
  const onAccessEndChange = jest.fn();

  // Execute.
  render(
    <Validation>
      <ScheduledAccessTiming
        start={start}
        maxAllowedDuration={8 * 60 * 60 * 1000}
        durationOptions={[]}
        selectedAccessEnd={{ value: end.getTime(), label: '2 hours' }}
        onStartChange={jest.fn()}
        onAccessEndChange={onAccessEndChange}
      />
    </Validation>
  );

  // Assert the editable controls and fixed duration are shown.
  expect(screen.getByText('Start Date')).toBeVisible();
  expect(screen.getByText('Start time (UTC+0)')).toBeVisible();
  expect(screen.getByText('End Date')).toBeVisible();
  expect(screen.getByText('End time (UTC+0)')).toBeVisible();
  expect(screen.getByText('Access Duration')).toBeVisible();
  expect(screen.getByText('2 hours')).toBeVisible();
  expect(screen.getAllByRole('combobox')).toHaveLength(2);
  expect(screen.getByText('4:00 PM (UTC)')).toBeVisible();
  expect(screen.getByText('6:00 PM (UTC)')).toBeVisible();

  // Execute an end-time change.
  const newEnd = new Date('2026-10-20T18:30:00Z');
  const endTimeSelect = screen.getAllByRole('combobox')[1];
  fireEvent.focus(endTimeSelect);
  fireEvent.keyDown(endTimeSelect, { key: 'ArrowDown', keyCode: 40 });
  fireEvent.click(screen.getByText(displayTimeWithTimeZone(newEnd)));

  // Assert the callback receives the new absolute end and duration.
  expect(onAccessEndChange).toHaveBeenCalledWith({
    value: newEnd.getTime(),
    label: getFormattedDurationTxt({ start, end: newEnd }),
  });
});
